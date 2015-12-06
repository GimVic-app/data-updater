export GOPATH=/home/zigapk/Programing/Go/ #modify to match your GOPATH
go get github.com/go-sql-driver/mysql
go get github.com/jinzhu/now
go build src/main.go
mv main /usr/bin/gimvic-data-updater
chown root /usr/bin/gimvic-data-updater
chmod 777 /usr/bin/gimvic-data-updater

