# Lazy soap

[![Docker Cloud Automated build](https://img.shields.io/docker/cloud/automated/nikscorp/soap)](https://hub.docker.com/repository/docker/nikscorp/soap)
[![Docker Image CI](https://github.com/Nikscorp/soap/workflows/Docker%20Image%20CI/badge.svg?branch=master)](https://github.com/Nikscorp/soap/actions)

A simple website to get the best episodes of favorite TV series. It may be helpful if you want to watch series with completely standalone episodes like "Black Mirror" as well as refresh some of the favorites series and save time.

## Usage

It's publicly available at [https://nikscorp.com](https://nikscorp.com). You also can host your instance by building/pulling the image or building binary from sources.
Note that frontend statics included in the Docker image and hosted by the application itself.

You can use frontend based version [https://nikscorp.com](https://nikscorp.com) or use json api directly. There are only 2 methods:

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
2. Change password in `etc/redis.conf.sample` to smth strong and rename file to `etc/redis.conf`
3. Consider changing settings in `docker-compose.yml` like listen IP/port and logging.
4. Run `docker-compose pull`
5. Set environment variables:
   - `API_KEY` to your OMDB API KEY
   - `REDIS_PASSWD` to your Redis password
6. Run `docker-compose up -d`
7. Consider setting up some https reverse proxy if you want to use it in public networks.

### Build a custom image

The same steps, but instead of step 4 run `docker-compose build`

### Build from sources

Follow the logic described in Dockerfile

## Additional information

## Caching

As OMDB API is quite slow and has some request restrictions, I use Redis for caching the responses. Max cache size is 100Mb and configurable in `etc/redis.conf`. Caching time is a week for now and not configurable as it seems to be the best candidate for the average time of new episode arrival.

## What with all that HA in compose file

I did this project as a task for my Cloud Computing course with requirements to provide basic HA configuration for Docker Swarm mode.

You can try this configuration by running:

`docker stack deploy -c docker-compose.yml soap`

Consider changing the placement of the replicas and count.
