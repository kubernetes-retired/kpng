#! /bin/ash
# usage: see hack/dev-docker

# uid arg
uid=0
if [ $# -gt 0 ]; then
    uid=$1
    shift

    adduser -h /home/user -s /bin/ash -u $uid -D -H user
    chown -R user: /go /home/user
fi

cd /src
exec sudo -u user -s "$@"
