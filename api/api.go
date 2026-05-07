package api

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"fmt"
	"intranet/database"
	"math"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

var db *sql.DB

type Correspondent struct {
	ID   *int   `json:"id,omitempty"`
	Name string `json:"name"`
}

type Sale struct {
	ID            *int    `json:"id,omitempty"`
	Correspondent int     `json:"correspondent"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
}

type ReturningSale struct {
	ID              *int    `json:"id,omitempty"`
	Correspondent   int     `json:"correspondent"`
	Name            string  `json:"name"`
	Value           float64 `json:"value"`
	ReturningMonths int16   `json:"returning_months"`
}

func Main(addr string, port string) {
	db = database.DB

	r := gin.Default()
	r.Use(cors.Default())
	r.Use(SSHAuthMiddleware())

	r.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Ehlo"})
	})

	r.GET("/correspondent", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, name FROM correspondent`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var correspondents []Correspondent
		for rows.Next() {
			var data Correspondent
			if err := rows.Scan(&data.ID, &data.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			correspondents = append(correspondents, data)
		}

		c.JSON(http.StatusOK, correspondents)
	})
	r.GET("/correspondent/byID/:id", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, name FROM correspondent WHERE id = ?`, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var correspondents []Correspondent
		for rows.Next() {
			var data Correspondent
			if err := rows.Scan(&data.ID, &data.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			correspondents = append(correspondents, data)
		}

		c.JSON(http.StatusOK, correspondents)
	})
	r.GET("/correspondent/byName/:name", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, name FROM correspondent WHERE name = ?`, strings.ReplaceAll(c.Param("name"), "_whitespace_", " "))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var correspondents []Correspondent
		for rows.Next() {
			var data Correspondent
			if err := rows.Scan(&data.ID, &data.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			correspondents = append(correspondents, data)
		}

		c.JSON(http.StatusOK, correspondents)
	})
	r.POST("/correspondent", func(c *gin.Context) {
		var data Correspondent
		if err := c.BindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.QueryRow(
			`INSERT INTO correspondent (name) VALUES (?) returning id`,
			data.Name,
		).Scan(&data.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, data)
	})

	r.POST("/sales/returning", func(c *gin.Context) {
		var data ReturningSale
		if err := c.BindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if _, err := db.Exec(
			`INSERT INTO sales_regulary (correspondent, name, value, returning_months) VALUES (?, ?, ?, ?)`,
			data.Correspondent, data.Name, data.Value, data.ReturningMonths,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, data)
	})

	r.GET("/sales/returning/:id", func(c *gin.Context) {
		id := c.Param("id")
		row := db.QueryRow(
			`SELECT sales_regulary.id, correspondent AS correspondent_id, sales_regulary.name, value, returning_months, correspondent.name AS correspondent_name
			FROM sales_regulary
			JOIN correspondent ON sales_regulary.correspondent = correspondent.id
			WHERE sales_regulary.id = ?`,
			id)

		var data ReturningSale
		var correspondentName string
		if err := row.Scan(&data.ID, &data.Correspondent, &data.Name, &data.Value, &data.ReturningMonths, &correspondentName); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "sale not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		resp := gin.H{
			"id":                 data.ID,
			"correspondent_id":   data.Correspondent,
			"correspondent_name": correspondentName,
			"name":               data.Name,
			"value":              data.Value,
			"returning_months":   data.ReturningMonths,
			"monthly_cost":       calcCostForXMonthsRound(data.Value, data.ReturningMonths, 1),
			"yearly_cost":        calcCostForXMonthsRound(data.Value, data.ReturningMonths, 12),
		}

		c.IndentedJSON(http.StatusOK, resp)
	})

	r.GET("/sales/returning", func(c *gin.Context) {
		endpoints := make([]string, 0, len(database.TimeFrames))
		for endpoint := range database.TimeFrames {
			endpoints = append(endpoints, endpoint)
		}
		c.JSON(http.StatusOK, gin.H{"message": "please specify a time frame (e.g. /sales/returning/monthly) or all", "available_time_frames": endpoints})
	})

	for endpoint, months := range database.TimeFrames {
		months := int16(months)
		r.GET("/sales/returning/"+endpoint, func(c *gin.Context) {
			rows, err := db.Query(
				`SELECT sales_regulary.id, correspondent AS correspondent_id, sales_regulary.name, value, returning_months, correspondent.name AS correspondent_name
				FROM sales_regulary
				JOIN correspondent ON sales_regulary.correspondent = correspondent.id
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			var (
				resp              []gin.H
				rsd               ReturningSale
				correspondentName string
			)
			for rows.Next() {
				if err := rows.Scan(&rsd.ID, &rsd.Correspondent, &rsd.Name, &rsd.Value, &rsd.ReturningMonths, &correspondentName); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				resp = append(resp, gin.H{
					"id":                 rsd.ID,
					"correspondent_id":   rsd.Correspondent,
					"correspondent_name": correspondentName,
					"name":               rsd.Name,
					"cost":               calcCostForXMonthsRound(rsd.Value, rsd.ReturningMonths, months),
				})
			}

			c.JSON(http.StatusOK, resp)
		})

		r.GET("/sales/returning/"+endpoint+"/sum", func(c *gin.Context) {
			rows, err := db.Query(
				`SELECT value, returning_months
				FROM sales_regulary`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			var sum float64
			for rows.Next() {
				var value float64
				var returningMonths int16
				if err := rows.Scan(&value, &returningMonths); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				sum += calcCostForXMonths(value, returningMonths, months)
			}
			sum = math.Round(sum*100) / 100

			c.JSON(http.StatusOK, gin.H{"sum": sum})
		})
	}

	r.GET("/sales/returning/all", func(c *gin.Context) {
		rows, err := db.Query(
			`SELECT sales_regulary.id, correspondent AS correspondent_id, sales_regulary.name, value, returning_months, correspondent.name AS correspondent_name
			FROM sales_regulary
			JOIN correspondent ON sales_regulary.correspondent = correspondent.id
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var (
			resp              []gin.H
			rsd               ReturningSale
			correspondentName string
		)
		for rows.Next() {
			if err := rows.Scan(&rsd.ID, &rsd.Correspondent, &rsd.Name, &rsd.Value, &rsd.ReturningMonths, &correspondentName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			resp = append(resp, gin.H{
				"id":                 rsd.ID,
				"correspondent_id":   rsd.Correspondent,
				"correspondent_name": correspondentName,
				"name":               rsd.Name,
				"value":              rsd.Value,
				"returning_months":   rsd.ReturningMonths,
			})
		}

		c.JSON(http.StatusOK, resp)
	})

	r.Run(addr + ":" + port)
}

func calcCostForXMonths(value float64, returningMonths int16, months int16) float64 {
	return (value / float64(returningMonths)) * float64(months)
}

func calcCostForXMonthsRound(value float64, returningMonths int16, months int16) float64 {
	cost := calcCostForXMonths(value, returningMonths, months)
	return math.Round(cost*100) / 100
}

func SSHAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		signature := c.GetHeader("X-Signature")
		timestamp := c.GetHeader("X-Timestamp")
		userID := c.GetHeader("X-User-ID")

		// Validate required headers
		if signature == "" || timestamp == "" || userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication headers, required: X-Signature, X-Timestamp, X-User-ID"})
			c.Abort()
			return
		}

		// Fetch user's public key from database
		var storedPubKey string
		err := database.DB.QueryRow(`SELECT value FROM auth WHERE id = ?`, userID).Scan(&storedPubKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Parse SSH public key format to extract the actual crypto key
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 " + storedPubKey))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid public key format"})
			c.Abort()
			return
		}

		// Convert SSH public key to ed25519 public key
		cryptoPubKey := pubKey.(ssh.CryptoPublicKey).CryptoPublicKey()
		ed25519PubKey, ok := cryptoPubKey.(ed25519.PublicKey)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Only ed25519 keys are supported"})
			c.Abort()
			return
		}

		// Reconstruct the message that was signed (must match client side)
		message := fmt.Sprintf("%s|%s|%s", c.Request.Method, c.Request.URL.Path, timestamp)

		// Decode signature from base64 header value
		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature encoding"})
			c.Abort()
			return
		}

		// Verify the signature
		if !ed25519.Verify(ed25519PubKey, []byte(message), sigBytes) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			c.Abort()
			return
		}

		// Store user ID in context for downstream handlers
		c.Set("user_id", userID)
		c.Next()
	}
}
