.PHONY: member
member:
	go build -o agent cmd/agent.go

.PHONY: clean
clean:
	rm -f agent 

