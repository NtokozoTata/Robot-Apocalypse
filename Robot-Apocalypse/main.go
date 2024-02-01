package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// Survivor represents a survivor in the database.
type Survivor struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Age               int       `json:"age"`
	Gender            string    `json:"gender"`
	LocationID        int       `json:"location_id"`
	Infected          bool      `json:"infected"`
	Location          Location  `json:"location"`
	Inventory         Inventory `json:"inventory"`
	ContaminationFlag bool      `json:"contamination_flag"`
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
INSERT INTO survivors (name, age, gender, location_id, infected, water, food, medication, ammunition, contamination_flag)
VALUES ($1, $2, $3, (
	SELECT id FROM locations WHERE latitude = $4 AND longitude = $5
), $6, $7, $8, $9, $10, $11)
RETURNING id, name, age, gender, location_id, infected, water, food, medication, ammunition, contamination_flag;
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
		newSurvivor.ContaminationFlag,
	).Scan(
		&newSurvivor.ID, &newSurvivor.Name, &newSurvivor.Age, &newSurvivor.Gender,
		&newSurvivor.LocationID, &newSurvivor.Infected,
		&newSurvivor.Inventory.Water,
		&newSurvivor.Inventory.Food,
		&newSurvivor.Inventory.Medication,
		&newSurvivor.Inventory.Ammunition,
		&newSurvivor.ContaminationFlag,
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

func updateSurvivorInventoryHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	var inventoryUpdate Inventory
	if err := c.ShouldBindJSON(&inventoryUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if err := updateSurvivorInventory(survivorID, inventoryUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor inventory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Survivor inventory updated successfully"})
}

func flagSurvivorAsInfectedHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	isInfected, err := checkSurvivorInfectionStatus(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check survivor infection status"})
		return
	}

	if isInfected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Survivor is already infected"})
		return
	}

	err = incrementContaminationReports(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment contamination reports"})
		return
	}

	thresholdReached, err := checkContaminationThreshold(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contamination threshold"})
		return
	}

	if thresholdReached {
		if err := flagSurvivorAsInfected(survivorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor infection status"})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

func flagSurvivorAsInfected(survivorID int) error {
	_, err := db.Exec("UPDATE survivors SET infected = true WHERE id = $1", survivorID)
	return err
}

func checkSurvivorInfectionStatus(survivorID int) (bool, error) {
	var isInfected bool
	err := db.QueryRow("SELECT infected FROM survivors WHERE id = $1", survivorID).Scan(&isInfected)
	return isInfected, err
}

func incrementContaminationReports(survivorID int) error {
	_, err := db.Exec("UPDATE survivors SET contamination_reports = contamination_reports + 1 WHERE id = $1", survivorID)
	return err
}

func checkContaminationThreshold(survivorID int) (bool, error) {
	var contaminationReports int
	err := db.QueryRow("SELECT contamination_reports FROM survivors WHERE id = $1", survivorID).Scan(&contaminationReports)
	return contaminationReports >= 3, err
}

func getRobotsHandler(c *gin.Context) {
	robots, err := fetchRobots()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch robots"})
		return
	}

	sort.Slice(robots, func(i, j int) bool {
		return robots[i].Category < robots[j].Category
	})

	c.JSON(http.StatusOK, robots)
}

func fetchRobots() ([]Robot, error) {
	apiURL := "https://robotstakeover20210903110417.azurewebsites.net/robotcpu"

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var robots []Robot
	err = json.NewDecoder(resp.Body).Decode(&robots)
	if err != nil {
		return nil, err
	}

	for _, robot := range robots {
		if robot.Category == "Flying" {
			robot.Category = "Land"
		}
	}

	return robots, nil
}

func getSurvivorPercentagesHandler(c *gin.Context) {
	infectedCount, totalCount, infectedSurvivors, nonInfectedSurvivors, err := querySurvivorPercentagesWithSurvivors()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying survivor percentages"})
		return
	}

	percentages := map[string]interface{}{
		"infected_count":         infectedCount,
		"total_count":            totalCount,
		"infected_survivors":     infectedSurvivors,
		"non_infected_survivors": nonInfectedSurvivors,
	}

	c.JSON(http.StatusOK, percentages)
}

func querySurvivorPercentagesWithSurvivors() (int, int, []Survivor, []Survivor, error) {
	var infectedCount, totalCount int
	var infectedSurvivors, nonInfectedSurvivors []Survivor

	query := `
		SELECT
			COUNT(*) FILTER (WHERE infected) AS infected_count,
			COUNT(*) AS total_count,
			ARRAY_AGG(survivors_infected) AS infected_survivors,
			ARRAY_AGG(survivors_non_infected) AS non_infected_survivors
		FROM (
			SELECT
				survivors.*,
				CASE WHEN infected THEN survivors END AS survivors_infected,
				CASE WHEN NOT infected THEN survivors END AS survivors_non_infected
			FROM survivors
		) AS subquery;
	`

	err := db.QueryRow(query).Scan(&infectedCount, &totalCount, pq.Array(&infectedSurvivors), pq.Array(&nonInfectedSurvivors))
	return infectedCount, totalCount, infectedSurvivors, nonInfectedSurvivors, err
}

func getInfectedSurvivorsHandler(c *gin.Context) {
	infectedSurvivors, err := querySurvivors(true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying infected survivors"})
		return
	}

	c.JSON(http.StatusOK, infectedSurvivors)
}

func getNonInfectedSurvivorsHandler(c *gin.Context) {
	nonInfectedSurvivors, err := querySurvivors(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying non-infected survivors"})
		return
	}

	c.JSON(http.StatusOK, nonInfectedSurvivors)
}

func submitInfectionReportHandler(c *gin.Context) {
	survivorID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid survivor ID"})
		return
	}

	err = incrementContaminationReports(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to increment contamination reports"})
		return
	}

	thresholdReached, err := checkContaminationThreshold(survivorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contamination threshold"})
		return
	}

	if thresholdReached {
		if err := flagSurvivorAsInfected(survivorID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update survivor infection status"})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

func querySurvivors(infected bool) ([]Survivor, error) {
	query := `
		SELECT
			s.id, s.name, s.age, s.gender, s.location_id, s.infected,
			l.latitude, l.longitude,
			i.water, i.food, i.medication, i.ammunition,
			s.contamination_reports
		FROM survivors s
		JOIN locations l ON s.location_id = l.id
		JOIN inventory i ON s.id = i.survivor_id
		WHERE s.infected = $1;
	`

	rows, err := db.Query(query, infected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var survivors []Survivor
	for rows.Next() {
		var survivor Survivor
		err := rows.Scan(
			&survivor.ID, &survivor.Name, &survivor.Age, &survivor.Gender, &survivor.LocationID, &survivor.Infected,
			&survivor.Location.Latitude, &survivor.Location.Longitude,
			&survivor.Inventory.Water, &survivor.Inventory.Food, &survivor.Inventory.Medication, &survivor.Inventory.Ammunition,
			&survivor.ContaminationFlag,
		)
		if err != nil {
			return nil, err
		}
		survivors = append(survivors, survivor)
	}

	return survivors, nil
}

func fetchRobots() ([]Robot, error) {
	apiURL := "https://robotstakeover20210903110417.azurewebsites.net/robotcpu"

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var robots []Robot
	err = json.NewDecoder(resp.Body).Decode(&robots)
	if err != nil {
		return nil, err
	}

	for _, robot := range robots {
		if robot.Category == "Flying" {
			robot.Category = "Land"
		}
	}

	return robots, nil
}

func main() {
	initDB()

	// Create a new Gin router
	r := gin.Default()

	// Define API routes
	r.POST("/survivors", registerSurvivorHandler)
	r.PUT("/survivors/:id/location", updateSurvivorLocationHandler)
	r.PUT("/survivors/:id/inventory", updateSurvivorInventoryHandler)
	r.POST("/survivors/:id/report", submitInfectionReportHandler)
	r.GET("/survivors/infected", getInfectedSurvivorsHandler)
	r.GET("/survivors/non_infected", getNonInfectedSurvivorsHandler)
	r.GET("/survivor_percentages", getSurvivorPercentagesHandler)
	r.GET("/robots", getRobotsHandler)

	// Run the Gin server
	r.Run(":8080")
}
