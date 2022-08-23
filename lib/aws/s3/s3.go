package s3

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/sts"
)

type S3 struct {
	cfg aws.Config
}

//https://aws.amazon.com/cn/blogs/china/use-sts-session-tags-to-perform-more-flexible-permission-control-on-aws-resources/
const (
	policy = `
	{
		"Version": "2012-10-17",
		"Statement": {
				"Effect": "Allow",
				"Principal": {
					"AWS": "arn:aws:iam::123456789012:user/Dave"
				},
				"Action": [
					"s3:PutObject"
				],
				"Resource": [
					"arn:aws:s3:::%s/%d/%s",
				]
		}
	}
	`
)

func Gen() {

	sess := session.New(&aws.Config{})
	stsInstance := sts.New(sess)
	var userId int64
	res, err := stsInstance.AssumeRole(&sts.AssumeRoleInput{
		DurationSeconds:   aws.Int64(100),
		ExternalId:        aws.String(fmt.Sprintf("_%d", userId)),
		Policy:            aws.String(``),
		PolicyArns:        []*sts.PolicyDescriptorType{},
		RoleArn:           aws.String(""),
		RoleSessionName:   aws.String(""),
		SerialNumber:      aws.String(""),
		Tags:              []*sts.Tag{},
		TokenCode:         aws.String(""),
		TransitiveTagKeys: nil,
	})
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	fmt.Printf("%+v\n", res.Credentials)
}
