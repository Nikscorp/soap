# Lazy Soap

A simple website to get the best episodes of favorite TV series. It may be helpful if you want to watch series with completely standalone episodes like "Black Mirror" as well as refresh some of the favorites series and save time.

## Usage

It's publicly available at [https://soap.nikscorp.com](https://soap.nikscorp.com). You also can host your instance by building/pulling the image or building binary from sources.
Note that frontend statics included in the Docker image and hosted by the application itself.

You can use frontend based version [https://soap.nikscorp.com](https://soap.nikscorp.com) or use json api directly. There are only 2 methods:

- `GET /search/{query}` to search id of series

```json
[
 {
 "imdbID": "tt2085059",
 "imdbRating": "",
 "poster": "https://m.media-amazon.com/images/M/MV5BYTM3YWVhMDMtNjczMy00NGEyLWJhZDctYjNhMTRkNDE0ZTI1XkEyXkFqcGdeQXVyMTkxNjUyNQ@@._V1_SX300.jpg",
 "title": "Black Mirror",
 "year": "2011\u2013"
 },
 ...
]
```

- `GET /id/{id}` to get the best episodes by id

```json
{
 "Episodes": [
 {
 "Episode": "2",
 "Season": "1",
 "Title": "Fifteen Million Merits",
 "floatRating": 8.1,
 "imdbRating": "8.1"
 },
 ...
 ],
 "Poster": "https://m.media-amazon.com/images/M/MV5BYTM3YWVhMDMtNjczMy00NGEyLWJhZDctYjNhMTRkNDE0ZTI1XkEyXkFqcGdeQXVyMTkxNjUyNQ@@._V1_SX300.jpg",
 "Title": "Black Mirror"
}
```

## Installation

To host your instance of app you follow these instructions.
To use any option you need to obtain OMDB API Key (free).

### Get pre-build image

1. Download `docker-compose.yml` and `etc/redis.conf.sample`
2. Consider changing settings in `docker-compose.yml` like listen IP/port and logging.
3. Pull the pre build image.
   - `docker-compose pull`
4. Set environment variables:
   - `API_KEY` to your OMDB API KEY
5. Start services.
   - `docker-compose up -d`
6. Consider setting up some https reverse proxy if you want to use it in public networks.

### Build a custom image

The same steps, but instead of step 4 run `docker-compose build`

### Build from sources

Follow the logic described in Dockerfile
