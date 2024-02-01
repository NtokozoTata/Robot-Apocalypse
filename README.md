## Robot Apocalypse API

## Introduction
Welcome to the Robot Apocalypse API documentation. This API allows you to manage survivors, their locations, inventories, and interact with various endpoints related to the Robot Apocalypse scenario.

## Table of Contents

- [Installation](#installation)
- [Dependencies](#dependencies)
- [Configuration](#configuration)
- [Database Setup](#database-setup)
- [Running the Application](#running-the-application)
- [API Endpoints](#api-endpoints)
- [Survivor Object](#survivor-object)
- [Location Object](#location-object)
- [Inventory Object](#inventory-object)
- [Robot Object](#robot-object)
- [Caching Robot Data](#caching-robot-data)

## Installation

To install and run the Robot Apocalypse API, follow these steps:

1. Clone the repository:

   ```bash
   git clone https://github.com/NtokozoTata/Robot-Apocalypse.git
   cd robot-apocalypse-api

2. Install the required dependencies:

    ```bash
    go mod download

3. Build the application:

    ```bash
    go build

## Dependencies
Gin: Web framework for building APIs.
gin-gonic/gin: Web framework for building the API.


PostgreSQL Driver: Database driver for PostgreSQL.
lib/pq: PostgreSQL driver for database interactions.

Sonic: Library for handling JSON encoding/decoding.
Others: Various utility libraries for handling HTTP requests, data manipulation, and testing.

Ensure that these dependencies are installed using go mod download before building the application.

## Configuration
database connection details
    dbUser := "postgres"
	dbPassword := "postgres"
	dbName := "Robot-Apocalypse"
	dbHost := "localhost"
	dbPort := "5432"

## Database Setup
Make sure you have PostgreSQL installed and running. Create a database named Robot-Apocalypse and execute the SQL scripts provided in the database directory to set up the required tables.

## Running the Application
Run the compiled binary: 
  -  ./robot-apocalypse-api
 or -  go run main.go

The API will start running on http://localhost:8080.

## API Endpoints

1.	Register Survivor Endpoint (THIS ONE WORKS)
        Method: POST
        URL: http://localhost:8080/register-survivor
        Headers: Content-Type: application/json
        Body: { "name": "John Doe", "age": 30, "gender": "Male", "location": { "latitude": 37.7749, "longitude": -122.4194 }, "infected": false, "inventory": { "water": "no", "food": "yes", "medication": "no", "ammunition": "yes" }, "contamination_flag": false }
//Will output error but it updates a new user on the database

2.	Update Survivor Inventory Endpoint (THIS ONE WORKS)
        Method: PUT
        URL: http://localhost:8080/survivors/{id}/update-inventory
        Headers: Content-Type: application/json
        Body: 
        {"water": "yes", "food": "no", "medication": "yes", "ammunition": "no"}

3.	Flag Survivor As Infected Endpoint (THIS ONE WORKS)
        Method: POST
        URL: http://localhost:8080/survivors/{id}/flag-infected
        Headers: Content-Type: application/json
//Do it 3 times for a user to be labeled as INFECTED

4.	Submit Infection Report Endpoint (THIS ONE WORKS)
        Method: POST
        URL: http://localhost:8080/survivors/{id}/submit-infection-report
        Headers: Content-Type: application/json

5.	Get Robots Endpoint (THIS ONE WORKS)
        Method: GET
        URL: http://localhost:8080/robots

6.	Get Survivor Percentages Endpoint (THIS ONE WORKS)
        Method: GET
        URL: http://localhost:8080/survivor-percentages

7.	Get Infected Survivors Endpoint
        Method: GET
        URL: http://localhost:8080/infected-survivors
8.	Get Non-Infected Survivors Endpoint
        Method: GET
        URL: http://localhost:8080/non-infected-survivors

## Caching Robot Data
The application caches robot data for performance. The cache duration is set to 5 minutes (configurable). Robot data is fetched from an external API endpoint (https://robotstakeover20210903110417.azurewebsites.net/robotcpu).

