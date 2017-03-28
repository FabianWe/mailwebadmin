all: default_theme.min.js default_bs.min.css

default_theme.min.js: default_theme.js
		yui-compressor default_theme.js -o default_theme.min.js

default_bs.min.css: default_bs.css
		yui-compressor default_bs.css -o default_bs.min.css
