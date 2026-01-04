# Libble
A Wordle inspired game where you guess the quotes from the books you've read.
Play the official version at [libble.you](https://libble.you)!

## Contributing

Feel free to make PRs. To get started working, clone the repo and install [air](https://github.com/air-verse/air) and Golang (>=1.25). 
Then run in dev mode with 
```bash
GIN_MODE=debug air -build.cmd=./build.sh -build.bin=./main
```

Take a look at the [to-do](./todo.md) for things that need to be done.

### Goals
1. Make a good game for your significant other that reads a lot
    - Really fast page loads and snappy UI, still keeping it simple and minimal.
2. Game should be fun and customizable, something they feel like they own and can use for the rest of their lives.
    - Minimize the need to connect to the server. 
        - You should only need to contact the server once when you just start or when you are trying to sync your data. Allows the game to be played offline or with no connection.
    - Have settings to customize the game.
4. Easy to pivot if Goodreads isn't available or changes their site.
    - Ideally can scrape from other sources
5. Easy to self host with just once instance (also makes developer experience better)

### Architecture

I'm a game dev and still pretty new to go and web world, so don't hesitate to make suggestions.

Both backend and frontend are written in go. A separate backend service is needed because you can't web-scrape on client due to CORS.
Backend server is mostly used for scraping and syncing player data. Hosted api.libble.(https://api.libble.you)
Actual frontend is just a static site using GitHub pages at [libble.you](https://libble.you).

#### Backend
- Using simple go [gin](https://github.com/gin-gonic/gin) HTTP server with a web-scraper using [colly](https://github.com/gocolly/colly). 
- Stores user data and scraped data for user's where their device can't store the amount of quotes they have. 
    - Currently it's in a SQLite database, but it needs to be moved to its own instance like Postgres. Started working on it in the `postgress-migration` branch. 
    - Need some kind of auth or verification so not everyone can use it.
- Dockerfile is for the hosting the server.

#### Frontend
- Using go WASM with a [DOM library](https://github.com/dominikh/go-js-dom) to do the game logic.
    - Thought it would be cool to do the whole thing in go.
    - I'm sure it'd be better to use React-TypeScript, golang DOM route hasn't been too bad.
