#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::Mutex;
use std::time::Duration;

use tauri::{Manager, RunEvent};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

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

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(Sidecar(Mutex::new(None)))
        .setup(|app| {
            let handle = app.handle().clone();

            // Spawn the bundled Go backend as a sidecar — unless something is
            // already serving :8002 (e.g. `beforeDevCommand` started it for
            // `tauri dev`, or a standalone backend is running). In that case we
            // reuse the existing backend instead of launching a second one.
            if wait_for_server("127.0.0.1", 8002, 1) {
                eprintln!("[cloud-manager] backend already listening on :8002, skipping sidecar spawn");
            } else {
                let sidecar = handle
                    .shell()
                    .sidecar("cloud-manager")
                    .map_err(|e| e.to_string())?;
                let (mut rx, child) = sidecar.spawn().map_err(|e| e.to_string())?;
                app.state::<Sidecar>().0.lock().unwrap().replace(child);

                // Forward sidecar stdout/stderr to the host process log.
                tauri::async_runtime::spawn(async move {
                    while let Some(event) = rx.recv().await {
                        match event {
                            CommandEvent::Stdout(bytes) => {
                                print!("[cloud-manager] {}", String::from_utf8_lossy(&bytes));
                            }
                            CommandEvent::Stderr(bytes) => {
                                eprint!("[cloud-manager] {}", String::from_utf8_lossy(&bytes));
                            }
                            _ => {}
                        }
                    }
                });
            }

            // The window starts hidden; reveal it once the backend is reachable.
            // Navigate explicitly after the port is open: the WebView may have
            // tried to load http://localhost:8002 at startup (before the sidecar
            // was listening) and gotten stuck on a blank "can't connect" page,
            // which would otherwise never reload on its own.
            let win = app.get_webview_window("main");
            std::thread::spawn(move || {
                let ready = wait_for_server("127.0.0.1", 8002, 100);
                if let Some(w) = win {
                    if ready {
                        let _ = w.navigate(
                            url::Url::parse("http://localhost:8002").unwrap(),
                        );
                    }
                    let _ = w.show();
                    let _ = w.set_focus();
                }
            });

            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while running tauri application")
        .run(|app, event| {
            // 应用退出时(含窗口关闭)杀掉本进程拉起的 sidecar 后端,避免孤儿进程。
            // 复用的外部后端(如 tauri dev 的 beforeDevCommand 所启)不在状态里,不受影响。
            if let RunEvent::Exit = event {
                if let Some(child) = app.state::<Sidecar>().0.lock().unwrap().take() {
                    let _ = child.kill();
                }
            }
        });
}