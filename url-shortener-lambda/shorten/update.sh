set GOOS=linux

go build -o shorten main.go
zip deployment.zip shorten

aws lambda update-function-code --function-name ShortenFunction --region us-east-1 --zip-file fileb://./deployment.zip
