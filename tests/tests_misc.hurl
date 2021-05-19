GET http://{{TARGET}}:9000
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

POST http://{{TARGET}}:9000
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

UNKNOWN http://{{TARGET}}:9000
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

GET http://{{TARGET}}:9000/
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

POST http://{{TARGET}}:9000/
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

GET http://{{TARGET}}:9000/unknown
[Options]
http2: {{HTTP2}}
HTTP/1.1 404

GET http://{{TARGET}}:9000/status
[Options]
http2: {{HTTP2}}
HTTP/1.1 200
[Asserts]
body contains "OK"

POST http://{{TARGET}}:9000/status
[Options]
http2: {{HTTP2}}
HTTP/1.1 405
