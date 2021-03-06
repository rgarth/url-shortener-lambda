package main

import (
	"encoding/json"
	"net/http"
  "regexp"
  "strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/teris-io/shortid"
)

const (
	LinksTableName = "UrlShortenerLinks"
	Region         = "us-east-1"
)

type Request struct {
	URL string `json:"url"`
}

type Response struct {
	ShortURL string `json:"short_url"`
}

type Link struct {
	ShortURL string `json:"short_url"`
	LongURL  string `json:"long_url"`
}

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
  // Setup CORS header
  resp := events.APIGatewayProxyResponse{
    Headers: make(map[string]string),
  }
	resp.Headers["Access-Control-Allow-Origin"] = "*"
	// Parse request body
	rb := Request{}
	if err := json.Unmarshal([]byte(request.Body), &rb); err != nil {
		return resp, err
	}
  // Stop shortening self referential URLs
  matchStr := strings.ToLower(rb.URL)
  matched, err := regexp.MatchString("http(s?)://([a-z]+\\.)*shitp.st((/)+.*)*$", matchStr)
	if err != nil {
		return resp, err
	}
  if matched {
    resp.StatusCode = http.StatusNotAcceptable
    return resp, nil
  }

	// Start DynamoDB session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(Region),
	})
	if err != nil {
		return resp, err
	}
	svc := dynamodb.New(sess)

  link := Link{}
  // Lookup existing ShortURL
  var queryInput = &dynamodb.QueryInput{
    TableName:     aws.String(LinksTableName),
    IndexName:     aws.String("long_url-index"),
    Limit:         aws.Int64(1),
    KeyConditions: map[string]*dynamodb.Condition{
      "long_url": {
        ComparisonOperator: aws.String("EQ"),
        AttributeValueList:     []*dynamodb.AttributeValue{
          {
            S: aws.String(rb.URL),
          },
        },
      },
    },
  }
  result, err := svc.Query(queryInput)
  if err != nil {
    return resp, err
  }
  if (len(result.Items) > 0) {
    if err := dynamodbattribute.UnmarshalMap(result.Items[0], &link); err != nil {
      return events.APIGatewayProxyResponse{}, err
    }
  } else {
    // Generate short url
	  shortURL := shortid.MustGenerate()
	  // Because "shorten" endpoint is reserved
    for shortURL == "shorten" {
      shortURL = shortid.MustGenerate()
    }
    link.ShortURL = shortURL
    link.LongURL = rb.URL

    // Marshal link to attribute value map
    av, err := dynamodbattribute.MarshalMap(link)
    if err != nil {
      return resp, err
    }
    // Put link
    input := &dynamodb.PutItemInput{
      Item:      av,
      TableName: aws.String(LinksTableName),
    }
    if _, err = svc.PutItem(input); err != nil {
      return resp, err
    }
  }
	// Return short url
	response, err := json.Marshal(Response{ShortURL: link.ShortURL})
	if err != nil {
		return resp, err
	}
  resp.StatusCode = http.StatusOK
  resp.Body = string(response)

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
