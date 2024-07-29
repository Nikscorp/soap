# Lazy Soap

<div align="center">

[![Coverage Status](https://coveralls.io/repos/github/Nikscorp/soap/badge.svg?branch=master)](https://coveralls.io/github/Nikscorp/soap?branch=master)&nbsp;[![Build Status](https://github.com/Nikscorp/soap/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Nikscorp/soap/actions)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/Nikscorp/soap)](https://goreportcard.com/report/github.com/Nikscorp/soap)&nbsp;[![Health Status](https://gatus.nikscorp.dev/api/v1/endpoints/_soap-ping/health/badge.svg)](https://gatus.nikscorp.dev/endpoints/_soap-ping)

</div>


A simple website to get the best episodes of favorite TV series. It may be helpful if you want to watch series with completely standalone episodes like "Black Mirror" as well as refresh some of the favorites series and save time.

## Usage

It's publicly available at [https://soap.nivoynov.dev](https://soap.nivoynov.dev). You also can host your instance by building/pulling the image or building binary from sources.
Note that frontend statics included in the Docker image and hosted by the application itself.

You can use frontend based version [https://soap.nivoynov.dev](https://soap.nivoynov.dev) or use JSON API directly. There are only 2 methods:

- `GET /search/{query}` to search id of series

```json
{
	"searchResults": [
		{
			"id": 42009,
			"title": "Black Mirror",
			"firstAirDate": "2011-12-04",
			"poster": "https://image.tmdb.org/t/p/w92/7PRddO7z7mcPi21nZTCMGShAyy1.jpg",
			"rating": 8.3
		}
	],
	"language": "en"
}
```

- `GET /id/{id}?language=string` to get the best episodes by id

```json
{
	"episodes": [
		{
			"title": "The National Anthem",
			"rating": 7.504,
			"number": 1,
			"season": 1
		},
		{
			"title": "Fifteen Million Merits",
			"rating": 7.696,
			"number": 2,
			"season": 1
		},
		{
			"title": "Black Museum",
			"rating": 7.858,
			"number": 6,
			"season": 4
		}
	],
	"title": "Black Mirror",
	"poster": "https://image.tmdb.org/t/p/w92/7PRddO7z7mcPi21nZTCMGShAyy1.jpg"
}
```

## Installation

To host your instance of app you follow these instructions.
To use any option you need to obtain TMDB API Key (free).

### Get pre-build image

1. Download `docker-compose.yml` and `etc/redis.conf.sample`
2. Consider changing settings in `docker-compose.yml` like listen IP/port and logging.
3. Pull the pre build image.
   - `docker-compose pull`
4. Set environment variables:
   - `API_KEY` to your TMDB API KEY
5. Start services.
   - `docker-compose up -d`
6. Consider setting up some https reverse proxy if you want to use it in public networks.

### Build a custom image

The same steps, but instead of step 4 run `docker-compose build`

### Build from sources

Follow the logic described in Dockerfile
