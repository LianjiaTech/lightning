## [0.1.1] - 2018-01-12
### Added
- Implemented `socket.core.dns` module `getaddrinfo()`
- Implemented `socket.core` module `tcp4()` and `tcp6()`
- Implemented `socket{master}` userData object `bind()` and `listen()`

## [0.1.0] - 2017-10-14
### Added
- Fully implemented `mime.core` module in Go, which includes base64 and quoted-printable decoders & encoders
- Fully support `ltn12`, `mime`, `socket`, `socket.ftp`, `socket.headers`, `socket.smtp`, `socket.tp`, and `socket.url` modules by registering appropriate [LuaSocket](https://github.com/diegonehab/luasocket) sources
- Partially support `http` module `request()`, supporting "simple form" GET and POST, complete with SSL support
- Added experimental support of `socket` module `newtry()` and `protect()` using community [LuaSocket](https://github.com/diegonehab/luasocket) Lua sources
- Implemented `socket.core` module `connect()`, `gettime()`, `skip()`,  `sleep()`, and `tcp()` in Go
- Implemented `socket.core.dns` module `gethostname()` and `toip()` in Go
- Implemented `socket{client}` userData object `close()`, `getfd()`, `receive('*a')`, `receive('*l')`, `receive(<bytes>)`, `send()`, and `settimeout()` in Go
- Implemented `socket{master}` userData object `close()` (a no-op), `connect()`, and `settimeout()` in Go

<small>(formatted per [keepachangelog-1.1.0](http://keepachangelog.com/en/1.0.0/))</small>
