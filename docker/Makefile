docker: html mirage-ecs
	docker build -t mirage-ecs .

clean:
	rm -fr html/ mirage-ecs

html: ../html/*
	mkdir -p html
	cp -a ../html/* html/

mirage-ecs:
	cd .. && make clean && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make mirage-ecs
	cp ../mirage-ecs .
	rm -f ../mirage-ecs

