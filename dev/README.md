# How to setup local environment

## Resolve dependencies with golang/dep

```
$ docker-compose -f dev/tools.yml run --rm deps
```

## Run the application

```
$ docker-compose -f dev/runtime.yml up
```

Now you can get a '200 OK' response from `http://localhost:9000/version` .
