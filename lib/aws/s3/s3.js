AWS.config.update({
    region: "us-east-1",
    credentials: new AWS.Credentials('AccessKeyId', 'SecretAccessKey', 'SessionToken')
});

var s3 = new AWS.S3();
var params = {
    Body: "The quick brown fox jumps over the lazy dog",
    Bucket: "example-bucket",
    Key: "hello.txt"
};
s3.putObject(params, function (err, data) {
    if (err) console.log(err, err.stack);
    else console.log(data);
});