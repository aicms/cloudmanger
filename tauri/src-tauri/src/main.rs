#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::Mutex;
use std::time::Duration;

use tauri::Manager;
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

/// Holds the spawned sidecar child process for the whole app lifetime.
/// Dropping `CommandChild` kills the process, so we keep it in managed state.
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

            // Spawn the bundled Go backend as a sidecar.
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
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
