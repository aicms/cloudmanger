#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use tauri::{Manager, RunEvent};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

const BACKEND_HOST: &str = "127.0.0.1";
const BACKEND_PORT: u16 = 8002;
const BACKEND_URL: &str = "http://localhost:8002";

/// Holds the spawned sidecar child process for the whole app lifetime.
/// `CommandChild` has no Drop impl (killing is an explicit `kill(self)` call),
/// so we keep it here and kill it ourselves on `RunEvent::Exit`.
struct Sidecar(Mutex<Option<CommandChild>>);

/// Poll the backend TCP port until it accepts connections (or we give up).
fn wait_for_server(host: &str, port: u16, tries: usize) -> bool {
    for _ in 0..tries {
        if std::net::TcpStream::connect((host, port)).is_ok() {
            return true;
        }
        std::thread::sleep(Duration::from_millis(300));
    }
    false
}

/// Zoom the webview so UI elements fit the window: proportionally shrink
/// contents on narrow windows (design width 1360 logical px), keep 1.0 once
/// the window is wide enough. Text stays vector-crisp under WebView2 zoom.
fn fit_zoom(physical_width: u32, scale_factor: f64) -> f64 {
    let logical = physical_width as f64 / scale_factor.max(0.1);
    (logical / 1360.0).clamp(0.70, 1.0)
}

/// Apply fit_zoom, skipping redundant calls (resize storms).
fn apply_fit_zoom(w: &tauri::WebviewWindow) {
    static LAST_ZOOM_BITS: AtomicU64 = AtomicU64::new(0);
    let Ok(scale) = w.scale_factor() else { return };
    let Ok(size) = w.outer_size() else { return };
    let z = fit_zoom(size.width, scale);
    if LAST_ZOOM_BITS.swap(z.to_bits(), Ordering::Relaxed) != z.to_bits() {
        if let Err(e) = w.set_zoom(z) {
            eprintln!("[cloud-manager] set_zoom failed: {e}");
        }
    }
}

/// Percent-encode a whole string so it can be used as a data: URL payload.
fn percent_encode(s: &str) -> String {
    let mut out = String::with_capacity(s.len() * 2);
    for b in s.as_bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'.' | b'_' | b'~' => {
                out.push(*b as char)
            }
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}

/// Brand-styled inline page (loading / error) shown before the backend is up.
/// No external assets and no backend dependency, so it renders instantly.
fn page_html(status: &str, hint: &str, spinning: bool) -> String {
    // 外环(loading 时旋转的进度弧)+ 居中静止的云朵
    let ring = if spinning {
        r##"<svg class="ring" viewBox="0 0 48 48" fill="none">
          <circle cx="24" cy="24" r="22" stroke="rgba(159,208,255,.14)" stroke-width="2.5"/>
          <path d="M24 2a22 22 0 0 1 22 22" stroke="#4096ff" stroke-width="2.5" stroke-linecap="round"/>
        </svg>"##
    } else {
        r##"<svg class="ring" viewBox="0 0 48 48" fill="none">
          <circle cx="24" cy="24" r="22" stroke="rgba(255,120,117,.3)" stroke-width="2.5"/>
        </svg>"##
    };
    let cloud = if spinning {
        r##"<svg class="cloud" viewBox="0 0 24 24" fill="none">
          <path d="M17.5 15.5a4.5 4.5 0 1 0 0-9h-1.8A7 7 0 1 0 4 11.4" stroke="#9fd0ff" stroke-width="1.7" stroke-linecap="round"/>
        </svg>"##
    } else {
        r##"<svg class="cloud" viewBox="0 0 24 24" fill="none">
          <path d="M17.5 15.5a4.5 4.5 0 1 0 0-9h-1.8A7 7 0 1 0 4 11.4" stroke="#ff7875" stroke-width="1.7" stroke-linecap="round"/>
          <path d="M12 9.2v4.6" stroke="#ff7875" stroke-width="1.9" stroke-linecap="round"/>
          <circle cx="12" cy="16.8" r="0.5" fill="#ff7875"/>
        </svg>"##
    };
    let spin_css = if spinning { "animation:spin .9s linear infinite" } else { "" };
    format!(
        r##"<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8">
<style>
html{{height:100%}}
body{{margin:0;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:22px;
  background:radial-gradient(900px 480px at 25% 8%, rgba(64,150,255,.20) 0%, transparent 60%),
             linear-gradient(160deg, #0c2240 0%, #021a30 100%);
  color:#e8f1ff;font-family:-apple-system,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;
  user-select:none;overflow:hidden}}
.mark{{position:relative;width:92px;height:92px}}
.ring{{position:absolute;inset:0;width:92px;height:92px;{spin_css}}}
.cloud{{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);width:44px;height:44px}}
@keyframes spin{{to{{transform:rotate(360deg)}}}}
h1{{font-size:21px;font-weight:700;letter-spacing:1px;margin:0;text-align:center}}
h1 small{{display:block;font-size:12px;font-weight:400;color:#7f92b3;margin-top:6px;letter-spacing:.5px}}
.status{{font-size:13.5px;color:#9fd0ff}}
.dots::after{{content:'…';animation:dots 1.2s steps(4) infinite}}
@keyframes dots{{0%{{content:''}}25%{{content:'.'}}50%{{content:'..'}}75%{{content:'...'}}}}
.hint{{font-size:12px;color:#7f92b3;max-width:340px;text-align:center;line-height:1.7}}
</style></head><body>
<div class="mark">{ring}{cloud}</div>
<h1>CloudManager<small>多云管理平台</small></h1>
<div class="status {dot_cls}">{status}</div>
<div class="hint">{hint}</div>
</body></html>"##,
        ring = ring,
        cloud = cloud,
        spin_css = spin_css,
        dot_cls = if spinning { "dots" } else { "" },
        status = status,
        hint = hint,
    )
}

fn data_url(html: &str) -> url::Url {
    url::Url::parse(&format!(
        "data:text/html;charset=utf-8,{}",
        percent_encode(html)
    ))
    .expect("data url should parse")
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(Sidecar(Mutex::new(None)))
        .setup(|app| {
            let handle = app.handle().clone();
            let sidecar_dead = Arc::new(AtomicBool::new(false));

            // Spawn the bundled Go backend as a sidecar — unless something is
            // already serving :8002 (e.g. `beforeDevCommand` started it for
            // `tauri dev`, or a standalone backend is running). In that case we
            // reuse the existing backend instead of launching a second one.
            if wait_for_server(BACKEND_HOST, BACKEND_PORT, 1) {
                eprintln!("[cloud-manager] backend already listening on :8002, skipping sidecar spawn");
            } else {
                // Anchor the sidecar's working directory to the exe dir so
                // data/ and logs/ always land next to it (portable layout),
                // never in whatever CWD the app happened to be launched from.
                let exe_dir = std::env::current_exe()
                    .ok()
                    .and_then(|p| p.parent().map(|p| p.to_path_buf()));
                let sidecar = handle.shell().sidecar("cloud-manager").map_err(|e| e.to_string())?;
                let sidecar = match exe_dir {
                    Some(dir) => sidecar.current_dir(dir),
                    None => sidecar,
                };
                let (mut rx, child) = sidecar.spawn().map_err(|e| e.to_string())?;
                app.state::<Sidecar>().0.lock().unwrap().replace(child);

                // Forward sidecar stdout/stderr to the host process log, and
                // notice termination early so the UI can show an error instead
                // of polling a port that will never open.
                let dead = sidecar_dead.clone();
                tauri::async_runtime::spawn(async move {
                    while let Some(event) = rx.recv().await {
                        match event {
                            CommandEvent::Stdout(bytes) => {
                                print!("[cloud-manager] {}", String::from_utf8_lossy(&bytes));
                            }
                            CommandEvent::Stderr(bytes) => {
                                eprint!("[cloud-manager] {}", String::from_utf8_lossy(&bytes));
                            }
                            CommandEvent::Terminated(_) => {
                                dead.store(true, Ordering::Relaxed);
                            }
                            _ => {}
                        }
                    }
                });
            }

            // UX: show a branded loading page immediately instead of keeping
            // the window hidden (which felt like a dead double-click), then
            // switch to the real UI once the backend accepts connections.
            // NOTE: window ops must run on a spawned thread — calling show()
            // synchronously inside setup silently does nothing (the event loop
            // is not pumping yet), which left the window invisible for good.
            let win = app.get_webview_window("main");
            let backend_already_up = wait_for_server(BACKEND_HOST, BACKEND_PORT, 1);

            if let Some(w) = &win {
                let initial = if backend_already_up {
                    url::Url::parse(BACKEND_URL).unwrap()
                } else {
                    data_url(&page_html(
                        "正在启动本地服务",
                        "首次启动需要初始化数据库并同步云账号资源",
                        true,
                    ))
                };
                if let Err(e) = w.navigate(initial) {
                    eprintln!("[cloud-manager] initial navigate failed: {e}");
                }
            }

            let dead = sidecar_dead.clone();
            std::thread::spawn(move || {
                if let Some(ref w) = win {
                    if let Err(e) = w.show() {
                        eprintln!("[cloud-manager] show failed: {e}");
                    }
                    let _ = w.set_focus();
                    apply_fit_zoom(w);
                }
                if backend_already_up {
                    return;
                }
                // ~60s budget: local sidecar normally binds in 1-2s.
                let mut ready = false;
                for _ in 0..200 {
                    if dead.load(Ordering::Relaxed) {
                        break;
                    }
                    if std::net::TcpStream::connect((BACKEND_HOST, BACKEND_PORT)).is_ok() {
                        ready = true;
                        break;
                    }
                    std::thread::sleep(Duration::from_millis(300));
                }
                if let Some(w) = win {
                    let target = if ready {
                        url::Url::parse(BACKEND_URL).unwrap()
                    } else {
                        data_url(&page_html(
                            "后端服务启动失败",
                            "请关闭本窗口后重新打开应用;若多次失败,请检查端口 8002 是否被其他程序占用。",
                            false,
                        ))
                    };
                    if let Err(e) = w.navigate(target) {
                        eprintln!("[cloud-manager] final navigate failed: {e}");
                    }
                    apply_fit_zoom(&w); // re-apply: cross-origin nav may reset zoom
                }
            });

            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while running tauri application")
        .run(|app, event| {
            match event {
                // 应用退出时(含窗口关闭)杀掉本进程拉起的 sidecar 后端,避免孤儿进程。
                // 复用的外部后端(如 tauri dev 的 beforeDevCommand 所启)不在状态里,不受影响。
                RunEvent::Exit => {
                    if let Some(child) = app.state::<Sidecar>().0.lock().unwrap().take() {
                        let _ = child.kill();
                    }
                }
                // 窗口尺寸/DPI 变化时,让元素比例始终贴合窗口大小。
                RunEvent::WindowEvent { label, event: tauri::WindowEvent::Resized(_), .. } => {
                    if let Some(w) = app.get_webview_window(&label) {
                        apply_fit_zoom(&w);
                    }
                }
                RunEvent::WindowEvent {
                    label,
                    event: tauri::WindowEvent::ScaleFactorChanged { .. },
                    ..
                } => {
                    if let Some(w) = app.get_webview_window(&label) {
                        apply_fit_zoom(&w);
                    }
                }
                _ => {}
            }
        });
}
