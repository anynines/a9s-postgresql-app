# a9s PostgreSQL App

This is a sample app to check whether the a9s PostgreSQL service is working or not.

## Install, push and bind

Make sure you installed GO on your machine, [download this](https://golang.org/doc/install?download=go1.13.darwin-amd64.pkg) for mac.

Clone the repository
```
$ git clone https://github.com/anynines/a9s-postgresql-app
```

Create a service on the [a9s PaaS](https://paas.anynines.com)
```
$ cf create-service a9s-postgresql10 postgresql-single-small mypostgres
```

Push the app
```
$ cf push --no-start
```

Bind the app
```
$ cf bind-service postgres-app mypostgres
```

And start
```
$ cf start postgres-app
```

At last check the created url...


## Local test using Docker

To start it locally you should have Docker installed.
Afterwards just use the following command to create a PostgreSQL database and the postgresql application:

```
docker-compose up
```

If you made changes to the application itself, you can rebuild the docker image using

```
docker-compose build
```

## Remark

To bind the app to other PostgreSQL services than `a9s-postgresql10`, have a look at the `VCAPServices` struct.
