
crawler.go is a package, not a program on its own. 
for creating a working binary for crawler.go you need to change the package name to "main" and then run "go build ./cmd/crawler.go"

if you want to use it in the ./main.go file as a library, the package name needs to stay "crawler"

