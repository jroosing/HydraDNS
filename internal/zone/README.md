Zone parsing is implemented in this package to mirror the Python `hydradns/zone.py` behavior.

It supports `$ORIGIN`, `$TTL`, owner elision, and the RR types used by HydraDNS: A, AAAA, CNAME, MX, NS, TXT, SOA, PTR.
