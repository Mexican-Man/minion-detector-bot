package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	ProjectID = "xxx"
	Location  = "us-central1"
	ModelID   = "xxx"
)

var responses = []string{"<@%s> you just posted cringe.", "You're not funny, <@%s>.", "<@%s> 😠", "I know where you live <@%s>.", "<@%s> just stop.", "<@%s> that's even worse than the last...", "<@%s> it's time to stop.", "Watch it, <@%s>."}

const scoreThreshold = 0.7

const url string = "http://localhost/v1/models/default:predict"

func main() {
	discord, err := discordgo.New("Bot " + "ODIwNDg5MTE2MTY4NzQ5MDk2.YE16CQ.2Sb7HG8gqVrqf7f3vuRUQQvpVGQ")
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Set intents, so we get updates from Discord
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(messageCreate)
	discord.AddHandler(ready)

	// In this example, we only care about receiving message events.
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages)

	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()
}

// Ready event
func ready(s *discordgo.Session, e *discordgo.Ready) {
	// Set status
	s.UpdateGameStatus(0, "DELETE ALL MINIONS")
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Accumulate images
	var p payload

	// Discordgo sometimes doesnt load embeds immediately, so we wait and reassign
	time.Sleep(time.Millisecond * 1500)
	mes, _ := s.ChannelMessage(m.ChannelID, m.ID)

	// Check if no attachments or links
	if len(mes.Attachments) == 0 && len(mes.Embeds) == 0 {
		return
	}

	// Make temp folder
	os.RemoveAll("temp/")
	os.Mkdir("temp/", 0777)
	defer os.RemoveAll("temp/")

	// Iterate through attachments
	for _, el := range mes.Attachments {
		f := strings.ToLower(el.Filename)
		if strings.Contains(f, ".jpg") || strings.Contains(f, ".jpeg") || strings.Contains(f, ".png") {
			i, err := urlImgToB64(el.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			if i != (inst{}) { // Check if returned object is empty
				p.Instances = append(p.Instances, i)
			}
		} else if strings.Contains(f, ".mp4") || strings.Contains(f, ".gif") || strings.Contains(f, ".gifv") || strings.Contains(f, ".webm") {
			i, err := urlVidToB64(el.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			if i != nil { // Check if returned object is empty
				p.Instances = append(p.Instances, i...)
			}
		}
	}

	// Iterate through links
	for _, el := range mes.Embeds {
		if strings.Contains(strings.ToLower(el.URL), ".gif") {
			i, err := urlVidToB64(el.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			if len(i) != 0 { // Check if returned object is empty
				p.Instances = append(p.Instances, i...)
			}
		} else if el.Type == "video" || el.Video != nil {
			i, err := urlVidToB64(el.Video.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			if len(i) != 0 { // Check if returned object is empty
				p.Instances = append(p.Instances, i...)
			}
		} else if el.Type == "image" {
			// Temp fix for .gif instead of .gifv
			if strings.Contains(strings.ToLower(el.URL), ".gif") {
				// In case link is image
				i, err := urlImgToB64(el.Thumbnail.URL)
				if err != nil {
					log.Println(err)
					continue
				}
				if i != (inst{}) { // Check if returned object is empty
					p.Instances = append(p.Instances, i)
				}
				continue
			}

			// In case link is image
			i, err := urlImgToB64(el.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			if i != (inst{}) { // Check if returned object is empty
				p.Instances = append(p.Instances, i)
			}
		}
	}

	// Return if empty
	if len(p.Instances) == 0 {
		return
	}

	// Convert to JSON
	jsonValue, err := json.Marshal(p)
	if err != nil {
		log.Printf("unable to format JSON: %v", err)
		return
	}

	// Query GCR
	response, err := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Printf("unable to query: %v", err)
		s.UpdateGameStatus(1, "SHHHHH mom, I'm in class right now.") // Set idle to indicate bot isn't working.
		_, err = s.UserUpdateStatus("online")
		log.Println(err)
		return
	}
	s.UpdateGameStatus(0, "DELETE ALL MINIONS")
	defer response.Body.Close()
	if response.StatusCode != 200 {
		//log.Printf("%+v\n", response)
		return
	}

	// Parse response
	res, _ := ioutil.ReadAll(response.Body)
	var prediction preds
	err = json.Unmarshal(res, &prediction)
	if err != nil {
		log.Printf("unable to parse response: %v", err)
		log.Println(string(res))
		return
	}

	// Check each attachment
	for _, el := range prediction.Predictions {
		// Get index for minion label
		var label1 int
		for i, label := range el.Labels {
			if label == "minion" {
				label1 = i
			}
		}

		// Check score on each attachment
		log.Println(el.Key, el.Scores[label1])
		if el.Scores[label1] >= scoreThreshold {
			s.ChannelMessageSendReply(mes.ChannelID, fmt.Sprintf(randomResponse(), m.Author.ID), m.MessageReference) // Reply to message
			s.ChannelMessageDelete(mes.ChannelID, mes.ID)                                                            // Delete message

			// Save for later training
			input, _ := ioutil.ReadFile(el.Key)
			if err != nil {
				log.Println(err)
				continue
			}
			extractName := strings.TrimPrefix(el.Key, "temp/")
			ioutil.WriteFile(fmt.Sprintf("store/%.5f_%s", el.Scores[label1], extractName), input, 0644)
			return
		}
	}
}

func urlImgToB64(url string) (inst, error) {
	// Get image
	response, err := http.Get(url)
	if err != nil {
		log.Printf("unable to fetch image: %v", err)
		return inst{}, fmt.Errorf("unable to fetch image: %v", err)
	}
	defer response.Body.Close()

	// Convert to bytes
	res, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("unable to convert image to bytes: %v", err)
		return inst{}, fmt.Errorf("unable to convert image to bytes: %v", err)
	}

	// Convert to base64
	encoded := base64.StdEncoding.EncodeToString(res)
	return inst{Key: filepath.Base(url), ImageBytes: img{B64: encoded}}, nil
}

func urlVidToB64(url string) ([]inst, error) {
	var result []inst

	// Format URL
	r := regexp.MustCompile(`(media\d).giphy`)
	url = r.ReplaceAllString(url, "i.giphy")
	// Regex to try HTTP instead of HTTPS. Note that some platforms don't let you browse through HTTP.
	// Alternatively, leave this uncommented and compile FFMPEG with HTTPS support.
	/*
		r = regexp.MustCompile(`(https)`)
		url = r.ReplaceAllString(url, "http")
		split := strings.SplitN(url, "?", 2)
		url = split[0]
	*/

	// Get video length
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", url).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		return nil, fmt.Errorf("unable to probe length of video: %v", err)
	}
	length, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 32)
	if err != nil {
		return nil, fmt.Errorf("unable to parse length of video: %v", err)
	}

	// Cap max frames to 10
	times := int(length)
	if times > 10 {
		times = 10
	}

	for i := 0; i <= times; i++ {
		// Generate thumbnail png to test at start, middle, and end of video
		name := "temp/" + fmt.Sprintf("%d_%d.png", time.Now().UnixNano()/1e6, i)
		out, err := exec.Command("ffmpeg", "-ss", fmt.Sprint(i), "-i", url, "-vframes", "1", name).CombinedOutput()
		if err != nil {
			log.Println(string(out))
			log.Printf("unable to transcode video: %v", err)
			continue
		}

		// Read png to bytes
		res, err := ioutil.ReadFile(name)
		if err != nil {
			log.Printf("unable to read preview: %v", err)
			continue
		}

		// Convert to base64
		encoded := base64.StdEncoding.EncodeToString(res)
		result = append(result, inst{Key: name, ImageBytes: img{B64: encoded}})
	}

	return result, nil
}

func randomResponse() string {
	rand.Seed(time.Now().UnixNano())
	return responses[rand.Intn(len(responses))]
}

// Request objects

type img struct {
	B64 string `json:"b64"`
}

type inst struct {
	ImageBytes img    `json:"image_bytes"`
	Key        string `json:"key"`
}

type payload struct {
	Instances []inst `json:"instances"`
}

// Response objects

type pred struct {
	Labels []string  `json:"labels"`
	Scores []float32 `json:"scores"`
	Key    string    `json:"key"`
}

type preds struct {
	Predictions []pred `json:"predictions"`
}
