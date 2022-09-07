export $(grep -v '^#' .env | xargs)
GOOS=linux go build -o main main.go
build-lambda-zip.exe -output main.zip main .env
aws s3api put-object --bucket $BUCKET_NAME --key lambda/main.zip --body main.zip
aws lambda update-function-code --function-name $BUCKET_NAME-proxy --s3-bucket $BUCKET_NAME --s3-key lambda/main.zip
rm main
rm main.zip
