package main

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/joho/godotenv"
	"golang.org/x/exp/slices"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var uploader *s3manager.Uploader

type IndexCal struct {
	Name    string
	Events  int
	Updated time.Time
	URL     string
}

type TemplateData struct {
	IndexCals []IndexCal
}

func GetCourseNames(cal *ics.Calendar) []string {
	var names []string
	for _, v := range cal.Events() {
		loc := v.GetProperty(ics.ComponentPropertyLocation)
		if !slices.Contains(names, loc.Value) {
			names = append(names, loc.Value)
		}
	}
	return names
}

func GetUploader() *s3manager.Uploader {
	s3Config := &aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			os.Getenv("AWS_SESSION_TOKEN")),
	}

	session, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatal("error creating s3 session", err)
	}

	return s3manager.NewUploader(session)
}

func upload(key string, data string) {
	upInput := &s3manager.UploadInput{
		Bucket:      aws.String(os.Getenv("BUCKET_NAME")),
		Key:         aws.String(key),
		Body:        bytes.NewReader([]byte(data)),
		ACL:         aws.String("public-read"),
		ContentType: aws.String("html"),
	}
	res, err := uploader.UploadWithContext(context.Background(), upInput)
	if err != nil {
		log.Fatal("error uploading file to s3 bucket", err)
	}
	log.Println("uploaded file successfully", res.Location)
}

func handleRequest() (string, error) {
	err := godotenv.Load("env")
	if err != nil {
		log.Fatal("couldn't load .env", err)
	}

	resp, err := http.Get(os.Getenv("ICS_URL"))
	if err != nil {
		log.Fatal("couldn't make ics get request", err)
	}

	cal, err := ics.ParseCalendar(resp.Body)
	if err != nil {
		log.Fatal("couldn't parse response", err)
	}

	uploader = GetUploader()

	courses := GetCourseNames(cal)
	log.Println("found unique course calendars: ", len(courses))

	data := TemplateData{}

	for _, course := range courses {
		newcal := ics.NewCalendar()
		for _, v := range cal.Events() {
			loc := v.GetProperty(ics.ComponentPropertyLocation)
			if loc.Value == course {
				start, _ := v.GetStartAt()
				if start.Minute() == 59 {
					v.SetStartAt(start.Truncate(time.Hour))
					v.SetDuration(time.Minute * 59)
				}
				newcal.AddVEvent(v)
			}
		}
		log.Println(course)
		newcal.SetXWRCalName(course)
		newcal.SetName(course)
		newcal.SetProductId("d2l-ics-proxy")

		d := IndexCal{
			Name:    course,
			Events:  len(newcal.Events()),
			Updated: time.Now(),
			URL:     os.Getenv("PROXY_URL") + "cal-" + course + ".ics",
		}

		data.IndexCals = append(data.IndexCals, d)

		upload("cal-"+course+".ics", newcal.Serialize())
	}

	tmpl := template.Must(template.ParseFiles("index.html"))
	buffer := new(bytes.Buffer)
	tmpl.Execute(buffer, data)
	upload("index.html", buffer.String())

	return "sucessfully uploaded ics files to s3", nil
}

func main() {
	lambda.Start(handleRequest)
}
