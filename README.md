# Libble
A Wordle inspired game where you guess the quotes from the books you've read.
Play at [libble.you](https://libble.you)!

## Contributing

Feel free to make PRs. To get started working, clone the repo and install [air](https://github.com/air-verse/air) and golang. 
Then run in dev mode with ```GIN_MODE=debug air -build.cmd=./build.sh -build.bin=./main```

Take a look at the [todo](./todo.md) for things that need to be done.

### Architecture

I'm a game dev and still pretty new to go and web world, so don't hesitate to make suggestions.

#### The objectives I'm trying to hit are:
- Minimize the need to connect to the server. 
    - You should only need to contact the server once when you just start or when you are trying to sync your data. Allows the game to be played offline or with no connection.
- User can customize their experience and own their data, but shouldn't be overwhelming for someone just trying the game out.
    - Have settings to customize the game.
- Really fast page loads and snappy ui.

Both backend and frontend are written in go. Seperate backend service is needed because you can't webscrape on client due to CORS.
Backend server is mostly used for webscraping and syncing player data. Hosted at [api.libble.you](https://api.libble.you)
Actual frontend is just a static site using gitub pages at [libble.you](https://libble.you).

#### Backend
Using simple go [gin](https://github.com/gin-gonic/gin) http server with a webscraper using [colly](https://github.com/gocolly/colly). 
Dockerfile is for the hosting the api server.

#### Frontend
Using go WASM with a [DOM library](https://github.com/dominikh/go-js-dom) to do the game logic.
Thought it would be cool to do the whole thing in go. 
I'm sure it'd be better to use React-TypeScript, but wasn't interested in learning it. If you want to port it over, I gave AI a shot on the `react-port` branch but couldn't get it to build on my machine.









