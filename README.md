waybackd, find your way back !

# Usage

```sh
# ./waybackd -h
Usage of ./waybackd:
  -config string
    config file path (default "config.yaml")
  -setup
    request an OVH consumer key
```

## OVH API

1. Create an application at [https://www.ovh.com/auth/api/createApp](https://www.ovh.com/auth/api/createApp) to get an `application_key` and `application_secret`.

2. Add them to your config file along with your domains. Leave `consumer_key` empty for now (see [config.yaml.example](config.yaml.example)).

3. Run the setup to request a consumer key scoped to your configured domains:

```sh
./waybackd -setup
```

This prints a `consumer_key` and a validation URL to stdout. Open the URL in your browser, log in to your OVH account, and approve the request (select "Unlimited" validity to avoid expiration). Once validated, copy the `consumer_key` into your config file.

The following access rules are requested per domain:
* GET    /domain/zone/YOUR_DOMAIN_NAME/record
* POST   /domain/zone/YOUR_DOMAIN_NAME/record
* POST   /domain/zone/YOUR_DOMAIN_NAME/refresh
* GET    /domain/zone/YOUR_DOMAIN_NAME/record/*
* PUT    /domain/zone/YOUR_DOMAIN_NAME/record/*
* DELETE /domain/zone/YOUR_DOMAIN_NAME/record/*
