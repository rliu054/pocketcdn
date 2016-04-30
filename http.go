package main

import (
    // "time"
    "flag"
)

var (
    mirror = flag.String("mirror", "", "Mirror web base URL")
    logfile = flag.String("log", "-", "Set log file, default stdout")
    upstream = flag.String("upstream", "", "server base URL, conflict with -mirror")
)

