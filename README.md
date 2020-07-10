通过gitlab integrations拉取代码到目的文件夹或ftp推送至远程服务器

## build
```bash
export GO111MODULE=on
go build -o app main.go
```

## prod
```bash
cp config.yaml.tpl config.yaml
./app
```