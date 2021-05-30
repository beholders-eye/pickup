use actix_web::middleware::Logger;
use actix_web::{App, HttpServer};
use std::sync::mpsc;
use std::sync::mpsc::Sender;
use std::thread;

mod api;
mod app_state;
mod filemanager;
mod player;

use clap::{App as ClapApp, Arg};

use app_state::AppState;
use env_logger::Env;
use filemanager::{load, refresh, MusicDb};
use player::{Command, Player};

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    env_logger::Builder::from_env(Env::default().default_filter_or("info")).init();

    let args = ClapApp::new("pickup-rust")
        .about("your-app-description")
        .author("Andy O'Neill")
        .args(&[
            Arg::new("refresh")
                .about("Refresh the music files (can be slow on network drives)")
                .short('r')
                .long("refresh"),
            Arg::new("music_dir")
                .about("The root music directory")
                .short('d')
                .long("music-dir")
                .takes_value(true),
        ])
        .get_matches();

    let music_dir = args
        .value_of("music_dir")
        .unwrap_or("../music")
        .trim_end_matches('/');

    let files: MusicDb;
    if args.is_present("refresh") {
        files = refresh(String::from(music_dir)).unwrap();
    } else {
        files = load(String::from(music_dir)).unwrap();
    }
    log::info!("We have got {} files", files.len());

    let sender = spawn_player();

    log::info!("Starting on http://localhost:9090");
    HttpServer::new(move || {
        log::info!("Building app");
        App::new()
            .data(AppState {
                sender: sender.clone(),
            })
            .wrap(Logger::default())
            .service(api::hello)
            .service(api::control::play)
            .service(api::control::stop)
            .service(api::control::volume)
    })
    .bind("127.0.0.1:9090")?
    .shutdown_timeout(60) // <- Set shutdown timeout to 60 seconds
    .run()
    .await
}

fn spawn_player() -> Sender<Box<dyn Command>> {
    let (tx, rx) = mpsc::channel();

    thread::spawn(move || {
        let mut player = Player::new();

        for command in rx {
            player.command(command);
        }
    });
    tx
}
