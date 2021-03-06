## freeport

Requires Ruby 1.9+

Frees the ports specified on the command line by sending the processes a TERM signal.
Prints the PIDs of the processes being terminated.

    freeport 10000
    10000 - 986 987 989

## ppargs

Pretty prints all arguments passed into it. Great for debugging issues when passing args from shell scripts.

    ppargs --grep 'hello world' '*'
    72075 - 13:41:20.850 - ["--grep", "hello world", "*"]

## git-fetch-all

Fetches all Git repositories in your home folder. OS X only.

## watchps

Spawns `watch` to look for all processes with the given name. Updates continously.

    watch node

## jsondiff

*Requires jq*

Diffs two pretty-printed diff files.

    jsondiff /tmp/gas.json ./tmp/config/gas.json

## fix-open-with

Fixes OS X’s “Open With” context menu entries

    fix-open-with

## git-unpushed

Shows commits that weren’t jet pushed to the upstream branch
