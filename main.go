package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Survivor represents a survivor in the database.
type Survivor struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Age        int       `json:"age"`
	Gender     string    `json:"gender"`
	LocationID int       `json:"location_id"`
	Infected   bool      `json:"infected"`
	Location   Location  `json:"location"`
	Inventory  Inventory `json:"inventory"`
}

// Location represents the latitude and longitude of a survivor.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Inventory represents the resources a survivor owns.
type Inventory struct {
	Water      string `json:"water"`
	Food       string `json:"food"`
	Medication string `json:"medication"`
	Ammunition string `json:"ammunition"`
}

// RobotData holds cached robot data and its expiration time.
type RobotData struct {
	Robots     []Robot `json:"robots"`
	Expiration time.Time
}

// Robot represents a robot with its category and location.
type Robot struct {
	Category string   `json:"category"`
	Location Location `json:"location"`
}

var (
	db *sql.DB

	robotCache    RobotData
	cacheMutex    = &sync.Mutex{}
	cacheDuration = 5 * time.Minute // Configurable cache duration
)

func initDB() {
	// Connect to the database (replace with your credentials)
	dbUser := "postgres"
	dbPassword := "postgres"
	dbName := "Robot-Apocalypse"
	dbHost := "localhost"
	dbPort := "5432"

	dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		dbUser, dbPassword, dbName, dbHost, dbPort)

	var err error
	db, err = sql.Open("postgres", dbInfo)
	if err != nil {
		panic(err)
	}

	// Check the database connection
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Connected to the database")
}

// registerSurvivorHandler registers a new survivor.
func registerSurvivorHandler(c *gin.Context) {
	var newSurvivor Survivor

	if err := c.ShouldBindJSON(&newSurvivor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Insert the new survivor into the database
	insertSurvivorQuery := `
INSERT INTO survivors (name, age, gender, location_id, infected, water, food, medication, ammunition)
VALUES ($1, $2, $3, (
	SELECT id FROM locations WHERE latitude = $4 AND longitude = $5
), $6, $7, $8, $9, $10)
RETURNING id, name, age, gender, location_id, infected, water, food, medication, ammunition;
`

	// Execute the query with provided parameters
	err := db.QueryRow(
		insertSurvivorQuery,
		newSurvivor.Name, newSurvivor.Age, newSurvivor.Gender,
		newSurvivor.Location.Latitude, newSurvivor.Location.Longitude,
		newSurvivor.Infected,
		newSurvivor.Inventory.Water,
		newSurvivor.Inventory.Food,
		newSurvivor.Inventory.Medication,
		newSurvivor.Inventory.Ammunition,
	).Scan(
		&newSurvivor.ID, &newSurvivor.Name, &newSurvivor.Age, &newSurvivor.Gender,
		&newSurvivor.LocationID, &newSurvivor.Infected,
		&newSurvivor.Inventory.Water,
		&newSurvivor.Inventory.Food,
		&newSurvivor.Inventory.Medication,
		&newSurvivor.Inventory.Ammunition,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert new survivor into the database"})
		return
	}

	// Insert the inventory into the database
	insertInventoryQuery := `
        INSERT INTO inventory (survivor_id, water, food, medication, ammunition)
        VALUES ($1, $2, $3, $4, $5);
    `

	_, err = db.Exec(
		insertInventoryQuery,
		newSurvivor.ID,
		newSurvivor.Inventory.Water,
		newSurvivor.Inventory.Food,
		newSurvivor.Inventory.Medication,
		newSurvivor.Inventory.Ammunition,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert inventory for the survivor"})
		return
	}

	c.JSON(http.StatusCreated, newSurvivor)
}

func updateSurvivorLocationHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	var location Location
	if err := c.ShouldBindJSON(&location); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if err := updateSurvivorLocation(survivorID, location); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor location"})
		return
	}

	c.Status(http.StatusNoContent)
}

// updateSurvivorLocation updates the survivor's location.
func updateSurvivorLocation(survivorID int, location Location) error {
	// Prepare the SQL query to prevent SQL injection vulnerabilities
	updateStmt, err := db.Prepare("UPDATE survivors SET location_id = (SELECT id FROM locations WHERE latitude = $1 AND longitude = $2), latitude = $1, longitude = $2 WHERE id = $3")
	if err != nil {
		return err
	}
	defer updateStmt.Close() // Ensure the statement is closed

	// Log the query and parameters (for debugging)
	log.Println("Update query:", updateStmt)
	log.Println("Parameters:", location.Latitude, location.Longitude, survivorID)

	// Execute the query with provided parameters
	_, err = updateStmt.Exec(location.Latitude, location.Longitude, survivorID)
	if err != nil {
		return err
	}

	return nil // Indicate successful update
}

// updateSurvivorInventoryHandler updates the inventory of a survivor.
func updateSurvivorInventoryHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	// Bind JSON body to struct
	var inventoryUpdate Inventory
	if err := c.ShouldBindJSON(&inventoryUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Update survivor's inventory
	if err := updateSurvivorInventory(survivorID, inventoryUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor inventory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Survivor inventory updated successfully"})
}

// updateSurvivorInventory updates the survivor's inventory.
func updateSurvivorInventory(survivorID int, inventory Inventory) error {
	// Start a transaction to ensure atomicity
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback if there's an error

	// Update the survivors table
	updateSurvivorStmt, err := tx.Prepare("UPDATE survivors SET water = $1, food = $2, medication = $3, ammunition = $4 WHERE id = $5")
	if err != nil {
		return err
	}
	defer updateSurvivorStmt.Close()

	_, err = updateSurvivorStmt.Exec(inventory.Water, inventory.Food, inventory.Medication, inventory.Ammunition, survivorID)
	if err != nil {
		return err
	}

	// Check if the survivor already has an inventory entry
	var existingInventoryID int
	err = tx.QueryRow("SELECT id FROM inventory WHERE survivor_id = $1", survivorID).Scan(&existingInventoryID)

	if err != nil {
		// If no existing inventory entry, insert a new one
		insertInventoryStmt, err := tx.Prepare("INSERT INTO inventory (survivor_id, water, food, medication, ammunition) VALUES ($1, $2, $3, $4, $5)")
		if err != nil {
			return err
		}
		defer insertInventoryStmt.Close()

		_, err = insertInventoryStmt.Exec(survivorID, inventory.Water, inventory.Food, inventory.Medication, inventory.Ammunition)
		if err != nil {
			return err
		}
	} else {
		// If an existing inventory entry exists, update it
		updateInventoryStmt, err := tx.Prepare("UPDATE inventory SET water = $1, food = $2, medication = $3, ammunition = $4 WHERE id = $5")
		if err != nil {
			return err
		}
		defer updateInventoryStmt.Close()

		_, err = updateInventoryStmt.Exec(inventory.Water, inventory.Food, inventory.Medication, inventory.Ammunition, existingInventoryID)
		if err != nil {
			return err
		}
	}

	// Commit the transaction if everything is successful
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// flagSurvivorAsInfectedHandler flags the survivor as infected.
func flagSurvivorAsInfectedHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	// Check if the survivor is already infected
	isInfected, err := checkSurvivorInfectionStatus(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check survivor infection status"})
		return
	}

	if isInfected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Survivor is already infected"})
		return
	}

	// Increment the contamination report count
	err = incrementContaminationReports(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment contamination reports"})
		return
	}

	// Check if the contamination threshold is reached
	thresholdReached, err := checkContaminationThreshold(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contamination threshold"})
		return
	}

	// If the threshold is reached, flag the survivor as infected
	if thresholdReached {
		if err := flagSurvivorAsInfected(survivorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor infection status"})
			return
		}
	}

	c.Status(http.StatusNoContent) // No response body needed if successful
}

// flagSurvivorAsInfected flags a survivor as infected.
func flagSurvivorAsInfected(survivorID int) error {
	_, err := db.Exec("UPDATE survivors SET infected = true WHERE id = $1", survivorID)
	return err
}

// checkSurvivorInfectionStatus checks if a survivor is infected.
func checkSurvivorInfectionStatus(survivorID int) (bool, error) {
	var isInfected bool
	err := db.QueryRow("SELECT infected FROM survivors WHERE id = $1", survivorID).Scan(&isInfected)
	return isInfected, err
}

// incrementContaminationReports increments the contamination report count for a survivor.
func incrementContaminationReports(survivorID int) error {
	_, err := db.Exec("UPDATE survivors SET contamination_reports = contamination_reports + 1 WHERE id = $1", survivorID)
	return err
}

// checkContaminationThreshold checks if the contamination threshold is reached for a survivor.
func checkContaminationThreshold(survivorID int) (bool, error) {
	var contaminationReports int
	err := db.QueryRow("SELECT contamination_reports FROM survivors WHERE id = $1", survivorID).Scan(&contaminationReports)
	return contaminationReports >= 3, err
}

// getRobotsHandler returns the list of robots.
func getRobotsHandler(c *gin.Context) {
	robots, err := fetchRobots()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch robots"})
		return
	}

	// Sort the robots by category
	sort.Slice(robots, func(i, j int) bool {
		return robots[i].Category < robots[j].Category
	})

	c.JSON(http.StatusOK, robots)
}

// fetchRobots fetches the list of robots from a specified API endpoint.
func fetchRobots() ([]Robot, error) {
	// Replace this URL with the actual endpoint that provides robot data
	apiURL := "https://robotstakeover20210903110417.azurewebsites.net/robotcpu"

	// Make a GET request to the API
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response body into a slice of Robot
	var robots []Robot
	err = json.NewDecoder(resp.Body).Decode(&robots)
	if err != nil {
		return nil, err
	}

	// Adjust category names for better user understanding
	for _, robot := range robots {
		if robot.Category == "Flying" {
			robot.Category = "Land"
		}
	}

	return robots, nil
}

// getSurvivorPercentagesHandler returns the percentages of infected and non-infected survivors.
func getSurvivorPercentagesHandler(c *gin.Context) {
	var infectedCount, totalCount int

	query := `
		SELECT
			COUNT(*) FILTER (WHERE infected) * 100 / COUNT(*) AS infected_percentage,
			COUNT(*) FILTER (WHERE NOT infected) * 100 / COUNT(*) AS non_infected_percentage
		FROM survivors;
	`

	err := db.QueryRow(query).Scan(&infectedCount, &totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying survivor percentages"})
		return
	}

	percentages := map[string]float64{
		"infected":     float64(infectedCount),
		"non_infected": float64(totalCount - infectedCount),
	}

	c.JSON(http.StatusOK, percentages)
}

// getInfectedSurvivorsHandler returns the list of infected survivors.
func getInfectedSurvivorsHandler(c *gin.Context) {
	infectedSurvivors, err := querySurvivors(true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying infected survivors"})
		return
	}

	c.JSON(http.StatusOK, infectedSurvivors)
}

// getNonInfectedSurvivorsHandler returns the list of non-infected survivors.
func getNonInfectedSurvivorsHandler(c *gin.Context) {
	nonInfectedSurvivors, err := querySurvivors(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying non-infected survivors"})
		return
	}

	c.JSON(http.StatusOK, nonInfectedSurvivors)
}

// submitInfectionReportHandler submits an infection report for a survivor.
func submitInfectionReportHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	// Increment the contamination report count
	err = incrementContaminationReports(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment contamination reports"})
		return
	}

	// Check if the contamination threshold is reached
	thresholdReached, err := checkContaminationThreshold(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contamination threshold"})
		return
	}

	// If the threshold is reached, flag the survivor as infected
	if thresholdReached {
		if err := flagSurvivorAsInfected(survivorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor infection status"})
			return
		}
	}

	c.Status(http.StatusNoContent) // No response body needed if successful
}

// querySurvivors queries the database for survivors based on infection status.
func querySurvivors(infected bool) ([]Survivor, error) {
	rows, err := db.Query("SELECT * FROM survivors WHERE infected = $1", infected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var survivors []Survivor
	for rows.Next() {
		var survivor Survivor
		if err := rows.Scan(
			&survivor.ID, &survivor.Name, &survivor.Age, &survivor.Gender,
			&survivor.LocationID, &survivor.Infected,
			&survivor.Location.Latitude, &survivor.Location.Longitude,
			&survivor.Inventory.Water, &survivor.Inventory.Food, &survivor.Inventory.Medication, &survivor.Inventory.Ammunition,
		); err != nil {
			return nil, err
		}
		survivors = append(survivors, survivor)
	}

	return survivors, nil
}

func main() {
	// Initialize the database connection.
	initDB()

	// Create a new Gin router
	router := gin.Default()

	// Define API endpoints
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Welcome to the Robot Apocalypse API"})
	})

	router.POST("/survivors/:id/update-location", updateSurvivorLocationHandler)
	router.POST("/survivors/:id/update-inventory", updateSurvivorInventoryHandler)
	router.POST("/survivors/:id/flag-infected", flagSurvivorAsInfectedHandler)
	router.POST("/survivors/:id/submit-infection-report", submitInfectionReportHandler)
	router.GET("/robots", getRobotsHandler)
	router.GET("/survivor-percentages", getSurvivorPercentagesHandler)
	router.GET("/infected-survivors", getInfectedSurvivorsHandler)
	router.GET("/non-infected-survivors", getNonInfectedSurvivorsHandler)
	router.POST("/register-survivor", registerSurvivorHandler)

	// Run the Gin server
	port := 8080
	fmt.Printf("Server is running on http://localhost:%d\n", port)
	router.Run(fmt.Sprintf(":%d", port))
}
