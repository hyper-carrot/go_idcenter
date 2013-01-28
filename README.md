go_idcenter
===========

A simple implement of id center.

## Usage

1. Ensure the project in dir '$GOPATH/src'

2. Optional: 
   - Install [Redis](http://redis.io/) database.
   - Install [MySQL](http://www.mysql.com) database.

3. Get and install the library dependencies (Optional): 

```bash
# redis driver
go get github.com/garyburd/redigo/redis

# mysql driver
go get github.com/ziutek/mymysql/thrsafe
go get github.com/ziutek/mymysql/autorc
go get github.com/ziutek/mymysql/godrv

# go_lib
cd <$GOPATH1/src> # $GOPATH1 is the first part of $GOPATH.
git clone https://github.com/hyper-carrot/go_lib.git
```

4. Edit go_idcenter.config for your need.

5. Run:

To be continue... (The web server code is not ready.)

## License
 
Copyright (C) 2013

Distributed under the BSD-style license, the same as Go.