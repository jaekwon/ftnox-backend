## Install

### Conventions

* New structs are created with "StructName{", no space between the type & "{".
* Keep all SQL in the model files.

### Sass

* sudo apt-get install ruby1.9.1-full
* sudo gem install sass

### NginX

* sudo apt-get install nginx

### Ghetto deploy

* rsync --progress --exclude="python.env" --exclude=".sass-cache" -r .git/* allinbits:~/src/ftnox.com/.git/
* rsync --progress --exclude="python.env" --exclude=".git" --exclude=".sass-cache" -r . allinbits:~/src/ftnox.com

## Postgres

    sudo apt-get install python-software-properties
    sudo add-apt-repository ppa:pitti/postgresql
    sudo apt-get install postgresql-9.3 pgadmin3
    postgres -D /usr/local/var/postgres
    ( might be /usr/lib/postgresql/9.3/bin/postgres, might be that server had already started )
    /usr/local/bin/createuser -s postgres
    psql -Upostgres

    CREATE USER ftnox WITH PASSWORD 'ftnox';
    CREATE DATABASE ftnox;
    GRANT ALL PRIVILEGES ON DATABASE ftnox TO ftnox;

    psql -Uftnox -h localhost

## Bootstrap

* bootstrap scss files taken from https://github.com/twbs/bootstrap-sass
* javascript taken from https://github.com/twbs/bootstrap
* TODO: consider just migrating to LESS.

## Solvency

https://github.com/olalonde/blind-liability-proof
    npm install -g blproof
