
curl -X POST -d '{"grid": "28764618-aparna-manoj", "scrape_options": {"MinPersonalStars": 4, "MinQuoteLikes": 10, "MaxQuoteForBook":  50, "UseCache": true }}' \
    -H 'Content-Type: application/json' \
    http://localhost:8080/user/create
