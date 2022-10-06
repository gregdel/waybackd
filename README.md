waybackd, find your way back !

# Usage

```sh
# ./waybackd -h
Usage of ./waybackd:
  -clean
    cleanup dns records and exit
  -config string
    config file path (default "config.yaml")
  -daemon
    run in deamon mode
  -server
    run server mode
```

## OVH API

To generate the API tokens, go to [https://www.ovh.com/auth/api/createToken](https://www.ovh.com/auth/api/createToken).
You should use the "Unlimited" value on the Validy field to avoid the token expiration.
You should set the rights for **YOUR_DOMAIN_NAME** to:
* GET    /domain/zone/YOUR_DOMAIN_NAME/record
* POST   /domain/zone/YOUR_DOMAIN_NAME/record
* POST   /domain/zone/YOUR_DOMAIN_NAME/refresh
* GET    /domain/zone/YOUR_DOMAIN_NAME/record/*
* PUT    /domain/zone/YOUR_DOMAIN_NAME/record/*
* DELETE /domain/zone/YOUR_DOMAIN_NAME/record/*

Once you're done, you'll get 3 tokens to store in the configuration. See [config.yaml.example](config.yaml.example) for a default configuration.
