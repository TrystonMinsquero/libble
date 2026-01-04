- [ ] Make it easier to setup account
    - Currently if you have the Goodreads app installed it takes to you the app, which is hard to find your user id
    - Could have search function that scrapes search results for that user and show possible options
- [ ] Write tests for the scraper that run on a schedule (make sure the scraper doesn't become out of date)
- [ ] Have some sort of auth and user account.
    - Currently not secure at all. Anyone can login to user's data via Goodreads id.
    - No mitigations for a DDoS
- [ ] Have user/settings page
    - [ ] Can see stats
    - [ ] Can change colorscheme (also with custom css)
    - [ ] Can configure how quotes are selected (default of minimum likes of 5 can be higher or lower)
    - [ ] Can sync data across devices
        - [ ] Need to figure this out. Current options are:
            - Have other device for code to sync, only need to do once.
            - Or could have account, with cloud saves
- [ ] Think about having some sort of page system where you can view each book, quote, player, etc. on its own page

There are more todos in the source code, just search for `TODO:`
