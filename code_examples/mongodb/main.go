package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var collection *mongo.Collection

type Event struct {
	ID          string `json:"ID,omitempty" bson:"ID,omitempty"`
	Title       string `json:"Title,omitempty" bson:"Title,omitempty"`
	Description string `json:"Description,omitempty" bson:"Description,omitempty"`
}

type EventUpdate struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

func HomeLink(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome home!")
}

func CreateEvent(response http.ResponseWriter, request *http.Request) {
	fmt.Println("Starting CreateEvent Function...")
	response.Header().Set("content-type", "application/json")
	var newEvent Event
	err := json.NewDecoder(request.Body).Decode(&newEvent)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	insertResult, err := collection.InsertOne(context.TODO(), newEvent)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	json.NewEncoder(response).Encode(insertResult)
}

func GetOneEvent(response http.ResponseWriter, request *http.Request) {
	fmt.Println("Starting GetOneEvent Function...")
	response.Header().Set("content-type", "application/json")
	eventID := mux.Vars(request)["id"]
	fmt.Println("Event Id", eventID)
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	filter := bson.D{{"ID", eventID}}
	var result Event
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

func GetAllEvents(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("content-type", "application/json")
	var events []Event
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	if err = cursor.All(ctx, &events); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	json.NewEncoder(response).Encode(events)
}

func UpdateEvent(response http.ResponseWriter, request *http.Request) {
	fmt.Println("Starting UpdateEvent Function...")
	response.Header().Set("content-type", "application/json")
	eventID := mux.Vars(request)["id"]
	fmt.Println("Event Id", eventID)

	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	var event EventUpdate
	err := json.NewDecoder(request.Body).Decode(&event)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	filter := bson.D{{"ID", eventID}}
	update := bson.M{
		"$set": bson.M{"Title": event.Title, "Description": event.Description},
	}
	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	result := collection.FindOneAndUpdate(ctx, filter, update, &opt)
	if result.Err() != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + result.Err().Error() + `" }`))
		return
	}
	doc := bson.M{}
	_ = result.Decode(&doc)
	json.NewEncoder(response).Encode(doc)
}

func DeleteEvent(response http.ResponseWriter, request *http.Request) {
	fmt.Println("Starting UpdateEvent Function...")
	response.Header().Set("content-type", "application/json")
	eventID := mux.Vars(request)["id"]
	fmt.Println("Event Id", eventID)
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)

	filter := bson.D{{"ID", eventID}}

	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

func main() {
	fmt.Println("Starting the application...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://root:example@mongo:27017"))
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	collection = client.Database("synonyms").Collection("events")

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", HomeLink)
	router.HandleFunc("/event", CreateEvent).Methods("POST")
	router.HandleFunc("/events", GetAllEvents).Methods("GET")
	router.HandleFunc("/events/{id}", GetOneEvent).Methods("GET")
	router.HandleFunc("/events/{id}", UpdateEvent).Methods("PATCH")
	router.HandleFunc("/events/{id}", DeleteEvent).Methods("DELETE")

	http.ListenAndServe(":9090", router)
}

/*
type Event struct {
	ID          string `json:"ID,omitempty" bson:"ID,omitempty"`
	Title       string `json:"Title,omitempty" bson:"Title,omitempty"`
	Description string `json:"Description,omitempty" bson:"Description,omitempty"`
}

type EventUpdate struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
}

curl -X POST -d '{"ID": "1", "Title": "Title 1", "Description": "Description 1"}' http://localhost:9090/event

curl -X GET http://localhost:9090/events/1

*/
