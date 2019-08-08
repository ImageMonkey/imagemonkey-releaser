# Howto create a new imagemonkey release

* clone repository
* rename `.env.template` to `.env`
* set environment variables in `.env`
* make sure to set the `IMAGEMONKEY_VERSION` in the `.env` file appropriately
* build docker image with `docker-compose build`
* create a new release by running `docker-compose up`
