# ./fabio [![Build Status](https://travis-ci.org/eBay/fabio.svg?branch=master)](https://travis-ci.org/eBay/fabio)

##### Current version: 1.0.3

fabio is a fast, modern, zero-conf load balancing HTTP(S) router for deploying
microservices managed by consul.

It provides a single-binary alternative to running [consul-
template](https://github.com/hashicorp/consul-template) together with
haproxy/varnish/nginx/apache. Services provide one or more host/path prefixes
they serve and fabio updates the routing table every time a service becomes
(un-)available without restart.

fabio was developed at the [eBay Classifieds Group](http://www.ebayclassifiedsgroup.com)
in Amsterdam and is currently used to route traffic for
[marktplaats.nl](http://www.marktplaats.nl) and [kijiji.it](http://www.kijiji.it).
Marktplaats is running all of its traffic through fabio which is
several thousand requests per second distributed over several fabio
instances.

## Features

* Single binary in Go. No external dependencies.
* Zero-conf
* Hot-reloading of routing table through backend watchers
* Round robin and random distribution
* [Traffic Shaping](#Traffic Shaping) (send 5% of traffic to new instances)
* SSL client certificate authentication support
* Graphite metrics
* Request tracing
* WebUI
* Fast

## Documentation

* [Installation](#installation)
* [Quickstart](#quickstart)
* [Configuration](https://raw.githubusercontent.com/eBay/fabio/master/fabio.properties) (documented fabio.properties file)
* [Performance](#performance)
* [Service configuration](#service-configuration)
* [Manual overrides](#manual-overrides)
* [Routing](#routing)
* [Traffic shaping](#traffic-shaping)
* [Debugging](#debugging)
* [Request tracing](#request-tracing)
* [Web UI](#web-ui)
* [Changelog](https://github.com/eBay/fabio/blob/master/CHANGELOG.md)
* [License](#license)

## Quickstart

This is how you use fabio in your setup:

1. Register your service in consul
2. Register a **health check** in consul as described [here](https://consul.io/docs/agent/checks.html).
   Make sure the health check is **passing** since fabio will only watch services
   which have a passing health check.
3. Register one `urlprefix-` tag per `host/path` prefix it serves,
   e.g. `urlprefix-/css`, `urlprefix-/static`, `urlprefix-mysite.com/`
4. Start fabio without a config file (assuming a consul agent on `localhost:8500`)
   Watch the log output how fabio picks up the route to your service.
   Try starting/stopping your service to see how the routing table changes instantly.
5. Send all your HTTP traffic to fabio on port `9999`
6. Done

If you want fabio to handle SSL as well set the `proxy.addr` along with the
public/private key files in
[fabio.properties](https://github.com/eBay/fabio/blob/master/fabio.properties)
and run `fabio -cfg fabio.properties`. You might also want to set the
`proxy.header.clientip`, `proxy.header.tls` and `proxy.header.tls.value`
options.

Check the [Debugging](#debugging) section to see how to test fabio with `curl`.

See fabio in action

[![fabio demo](http://i.imgur.com/aivFAKl.png)](https://www.youtube.com/watch?v=gvxxu0PLevs"fabio demo - Click to Watch!")

## Installation

To install fabio run (you need Go 1.4 or higher)

    go get github.com/eBay/fabio

To start fabio run

    ./fabio

which will run it with the default configuration which is described
in `fabio.properties`. To run it with a config file run it
with

    ./fabio -cfg fabio.properties

or use the official Docker image and mount your own config file to `/etc/fabio/fabio.properties`

    docker run -d -p 9999:9999 -p 9998:9998 -v $PWD/fabio/fabio.properties:/etc/fabio/fabio.properties magiconair/fabio

If you want to run the Docker image with one or more SSL certificates then
you can store your configuration and certificates in `/etc/fabio` and mount
the entire directory, e.g.

    $ cat ~/fabio/fabio.properties
    proxy.addr=:443;/etc/fabio/ssl/mycert.pem;/etc/fabio/ssl/mykey.pem

    docker run -d -p 443:443 -p 9998:9998 -v $PWD/fabio:/etc/fabio magiconair/fabio

The official Docker image contains the root CA certificates from a recent and updated
Ubuntu 12.04.5 LTS installation.

## Performance

fabio has been tested to deliver up to 15.000 req/sec on a single 16
core host with moderate memory requirements (~ 60 MB).

To achieve the performance fabio sets the following defaults which
can be overwritten with the environment variables:

* `GOMAXPROCS` is set to `runtime.NumCPU()` since this is not the
  default for Go 1.4 and before
* `GOGC=800` is set to reduce the pressure on the garbage collector

When fabio is compiled with Go 1.5 and run with default settings it can be up
to 40% slower  than the same version compiled with Go 1.4. The `GOGC=100`
default puts more pressure on the Go 1.5 GC which makes the fabio spend 10% of
the time in the GC. With `GOGC=800` this drops back to 1-2%. Higher values
don't provide higher gains.

As usual, don't rely on these numbers and perform your own benchmarks. You can
check the time fabio spends in the GC with `GODEBUG=gctrace=1`.

## Service configuration

Each service can register one or more URL prefixes for which it serves
traffic. A URL prefix is a `host/path` combination without a scheme since SSL
has already been terminated and all traffic is expected to be HTTP. To
register a URL prefix add a tag `urlprefix-host/path` to the service
definition.

By default, traffic is distributed evenly across all service instances which
register a URL prefix but you can set the amount of traffic a set of instances
will receive ("Canary testing"). See [Traffic Shaping](#Traffic Shaping)
below.

A background process watches for service definition and health status changes
in consul. When a change is detected a new routing table is constructed using
the commands described in [Config Commands](#Config Commands).

## Manual overrides

Since an automatically generated routing table can only be changed with a
service deployment additional routing commands can be stored manually in the
consul KV store which get appended to the automatically generated routing
table. This allows fine-tuning and fixing of problems without a deployment.

The [Traffic Shaping](#Traffic Shaping) commands are also stored in the KV
store.

## Routing Table Configuration

The routing table is configured with the following commands:

```
route add service host/path targetURL [weight <weight>] [tags "tag1,tag2,..."]
	- Add a new route for host/path to targetURL

route del service
	- Remove all routes for service

route del service host/path
	- Remove all routes for host/path for this service only

route del service host/path targetURL
	- Remove only this route

route weight service host/path weight n tags "tag1,tag2"
  - Route n% of traffic to services matching service, host/path and tags
    n is a float > 0 describing a percentage, e.g. 0.5 == 50%
    n <= 0: means no fixed weighting. Traffic is evenly distributed
    n > 0: route will receive n% of traffic. If sum(n) > 1 then n is normalized.
    sum(n) >= 1: only matching services will receive traffic

```

The order of commands matters but routes are always ordered from most to least
specific by prefix length.

## Routing

The routing table contains first all routes with a host sorted by prefix
length in descending order and then all routes without a host again sorted by
prefix length in descending order.

For each incoming request the routing table is searched top to bottom for a
matching route. A route matches if either `host/path` or - if there was no
match - just `/path` matches.

The matching route determines the target URL depending on the configured
strategy. `rnd` and `rr` are available with `rnd` being the default.

### Example

The auto-generated routing table is

```
route add service-a www.mp.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-a www.kjca.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-a www.dba.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-b www.mp.dev/auth/ http://host-b:11080/ tags "a,b"
route add service-b www.kjca.dev/auth/ http://host-b:11080/ tags "a,b"
route add service-b www.dba.dev/auth/ http://host-b:11080/ tags "a,b"
```

The manual configuration under `/fabio/config` is

```
route del service-b www.dba.dev/auth/
route add service-c www.somedomain.com/ http://host-z:12345/
```

The complete routing table then is

```
route add service-a www.mp.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-a www.kjca.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-a www.dba.dev/accounts/ http://host-a:11050/ tags "a,b"
route add service-b www.mp.dev/auth/ http://host-b:11080/ tags "a,b"
route add service-b www.kjca.dev/auth/ http://host-b:11080/ tags "a,b"
route add service-c www.somedomain.com/ http://host-z:12345/ tags "a,b"
```

## Traffic Shaping

fabio allows to control the amount of traffic a set of service
instances will receive. You can use this feature to direct a fixed percentage
of traffic to a newer version of an existing service for testing ("Canary
testing").

The following command will allocate 5% of traffic to `www.kjca.dev/auth/` to
all instances of `service-b` which match tags `version-15` and `dc-fra`. This
is independent of the number of actual instances running. The remaining 95%
of the traffic will be distributed evenly across the remaining instances
publishing the same prefix.

```
route weight service-b www.kjca.dev/auth/ weight 0.05 tags "version-15,dc-fra"
```

## Debugging

To send a request from the command line via the fabio using `curl`
you should send it as follows:

```
curl -v -H 'Host: foo.com' 'http://localhost:9999/path'
```

The `-x` or `--proxy` options will most likely not work as you expect as they
send the full URL instead of just the request URI which usually does not match
any route but the default one - if configured.

## Request tracing

To trace how a request is routed you can add a `Trace` header with an non-
empty value which is truncated at 16 characters to keep the log output short.

```
$ curl -v -H 'Trace: abc' -H 'Host: foo.com' 'http://localhost:9999/bar/baz'

2015/09/28 21:56:26 [TRACE] abc Tracing foo.com/bar/baz
2015/09/28 21:56:26 [TRACE] abc No match foo.com/bang
2015/09/28 21:56:26 [TRACE] abc Match foo.com/
2015/09/28 22:01:34 [TRACE] abc Routing to http://1.2.3.4:8080/
```

## Web UI

fabio contains a (very) simple web ui to examine the routing
table. By default it is accessible on `http://localhost:9998/`

## License

MIT licensed

