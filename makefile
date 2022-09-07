rover-demo:
	go build -o rover-demo -ldflags="-w -s"
run: rover-demo
	sudo ./rover-demo