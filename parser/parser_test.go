package parser

import (
	"testing"

	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser/token"
	"gotest.tools/v3/assert"
)

func TestParser_CurrFollow(t *testing.T) {
	conf := `
	server { # simple reverse-proxy
	}
	`
	p, err := NewStringParser(conf)
	assert.NilError(t, err)
	//assert.Assert(t, tokens, 1)
	assert.Assert(t, p.curTokenIs(token.Keyword))
	assert.Assert(t, p.followingTokenIs(token.BlockStart))
}

//TODO(tufan): reactivate here after getting include specific things done
//func TestParser_Include(t *testing.T) {
//	conf := `
//	include /etc/ngin/conf.d/mime.types;
//	`
//	p := NewStringParser(conf)
//	c := p.Parse()
//	_, ok := c.Directives[0].(gonginx.IncludeDirective) //we expect the first statement to be an include
//	assert.Assert(t, ok)
//}

func TestParser_UnendedInclude(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	pa, err := NewParserFromLexer(
		lex(`
	server { 
	include /but/no/semicolon before block;
	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_LocationNoParam(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	pa, err := NewParserFromLexer(
		lex(`
	server { 
	location  {} #location with no param
	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_LocationTooManyParam(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	pa, err := NewParserFromLexer(
		lex(`
	server { 
	location one two three four {} #location with too many arguments
	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_ParseValidLocations(t *testing.T) {
	pa, err := NewParserFromLexer(
		lex(`
	server { 
		location  ~ /(.*)php/{

		} #location with no param

		location  /admin {

			} #location with no param

	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_ParseUpstream(t *testing.T) {
	pa, err := NewParserFromLexer(
		lex(`
		upstream my_upstream{
			server 127.0.0.1:8080;
			server 127.0.0.1:8080 weight=5 failure=3;
		}
	server { 
		location  ~ /(.*)php/{

		} #location with no param

		location  /admin {

			} #location with no param

	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_ParseFromFile(t *testing.T) {
	_, err := NewParser("../full-example/nginx.conf")
	assert.NilError(t, err)
	_, err2 := NewParser("../full-example/nginx.conf-not-found")
	assert.ErrorContains(t, err2, "no such file or directory")
}

func TestParser_MultiParamDirecive(t *testing.T) {
	pa, err := NewParserFromLexer(
		lex(`
http{
		server { 
			a_directive has multi params /and/ends;
			location ~ /and/ends{
				
			}
		}
}
	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func TestParser_Location(t *testing.T) {
	pa, err := NewParserFromLexer(
		lex(`
		location ~ /and/ends{
			
		} 
	`))
	assert.NilError(t, err)
	c, err := pa.Parse()
	assert.NilError(t, err)

	_, ok := c.Directives[0].(*gonginx.Location)
	assert.Assert(t, ok, "expecting a location as first statement")
}

func TestParser_VariableAsParameter(t *testing.T) {
	pa, err := NewParserFromLexer(
		lex(`
			map $host $clientname {
				default -;
			}
	`))
	assert.NilError(t, err)
	c, err := pa.Parse()
	assert.NilError(t, err)

	d, ok := c.Directives[0].(*gonginx.Directive)
	assert.Assert(t, ok, "expecting a directive(http) as first statement")
	assert.Equal(t, d.Name, "map", "first directive needs to be ")
	assert.Equal(t, len(d.Parameters), 2, "map must have 2 parameters here")
	assert.Equal(t, d.Parameters[0], "$host", "invalid first parameter")
	assert.Equal(t, d.Parameters[1], "$clientname", "invalid second parameter")
}

func TestParser_UnendedMultiParams(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	pa, err := NewParserFromLexer(
		lex(`
	server { 
	a_driective with mutli params /but/no/semicolon/to/panic }
	`))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
}

func Benchmark_ParseFullExample(t *testing.B) {
	fullconf := `user www www;
worker_processes 5;
error_log logs/error.log;
pid logs/nginx.pid;
worker_rlimit_nofile 8192;
events { worker_connections 4096; } http {
include mime.types;
include proxy.conf;
include fastcgi.conf;
index index.html index.htm index.php;
default_type application/octet-stream;
log_format main '$remote_addr - $remote_user [$time_local]  $status '  
'"$request" $body_bytes_sent "$http_referer" '
' "$http_user_agent" "$http_x_forwarded_for"';
access_log logs/access.log main;
sendfile on;
tcp_nopush on;
server_names_hash_bucket_size 128;
server {
listen 80;
server_name domain1.com www.domain1.com;
access_log logs/domain1.access.log main;
root html;
location ~ \.php$ {
fastcgi_pass 127.0.0.1:1025; } } server {
listen 80;
server_name domain2.com www.domain2.com;
access_log logs/domain2.access.log main;
location ~ ^/(images|javascript|js|css|flash|media|static)/ {
root /var/www/virtual/big.server.com/htdocs;
expires 30d;
} location / { proxy_pass http://127.0.0.1:8080; } }
upstream big_server_com {
server 127.0.0.3:8000 weight=5;
server 127.0.0.3:8001 weight=5;
server 192.168.0.1:8000;
server 192.168.0.1:8001;
} server { listen 80;
server_name big.server.com;
access_log logs/big.server.access.log main;
location / { proxy_pass http://big_server_com; } } }`
	for n := 0; n < t.N; n++ {
		pa, err := NewParserFromLexer(
			lex(fullconf))
	assert.NilError(t, err)
	_, err = pa.Parse()
	assert.NilError(t, err)
	}
}
