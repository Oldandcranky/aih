package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Databingo/EdgeGPT-Go"
	"github.com/Databingo/aih/eng"
	"github.com/Databingo/googleBard/bard"
	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/peterh/liner"
	"github.com/rocketlaunchr/google-search"
	openai "github.com/sashabaranov/go-openai"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func clear() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func multiln_input(Liner *liner.State, prompt string) string {
	// For recognize multipile lines input module
	// |------------------------|------
	// |recording && input      | action
	// |------------------------|------
	// |false && head == "" or x| record; break
	// |false && head != "<<"   | record; break
	// |false && head == "<<"   | record; true; rm <<
	// |true  && head == "" or x| record;
	// |true  && end  != ">>"   | record;
	// |true  && end  == ">>"   | record; break; rm >>
	// |------------------------|------

	var ln string
	var lns []string
	recording := false
	for {
		if ln == "" && !recording {
			ln, _ = Liner.Prompt(prompt)
		} else {
			ln, _ = Liner.Prompt("")
		}
		ln = strings.Trim(ln, " ")
		if !recording && (ln == "" || len(ln) == 1) {
			lns = append(lns, ln)
			break
		} else if !recording && ln[:2] != "<<" {
			lns = append(lns, ln)
			break
		} else if !recording && ln[:2] == "<<" {
			recording = true
			lns = append(lns, ln[2:])
		} else if recording && (ln == "" || len(ln) == 1) {
			lns = append(lns, ln)
		} else if recording == true && ln[len(ln)-2:] != ">>" {
			lns = append(lns, ln)
		} else if recording == true && ln[len(ln)-2:] == ">>" {
			recording = false
			lns = append(lns, ln[:len(ln)-2])
			break
		}
	}

	long_str := strings.Join(lns, "\n")
	return long_str
}

func main() {
	// Create prompt for user input
	Liner := liner.NewLiner()
	defer Liner.Close()

	// Read user input history
	if f, err := os.Open(".history"); err == nil {
		Liner.ReadHistory(f)
		f.Close()
	}

	// Use RESP for record response per time per time
	var RESP string

	// Read Aih Configure
	aih_json, err := ioutil.ReadFile("aih.json")
	if err != nil {
		err = ioutil.WriteFile("aih.json", []byte(""), 0644)
	}

	// Read Proxy
	Proxy := gjson.Get(string(aih_json), "proxy").String()

	// Set proxy for system of current program
	os.Setenv("http_proxy", Proxy)
	os.Setenv("https_proxy", Proxy)

	// Test Proxy
TEST_PROXY:
	fmt.Println("Checking network accessing...")
	ops1 := googlesearch.SearchOptions{Limit: 12}
	_, err = googlesearch.Search(nil, "BTC", ops1)
	if err != nil {
		fmt.Println("Need proxy to access GoogleBard, BingChat, ChatGPT")
		proxy, _ := Liner.Prompt("Please input proxy: ")
		if proxy == "" {
			goto TEST_PROXY
		}
		aihj, err := ioutil.ReadFile("aih.json")
		new_aihj, _ := sjson.Set(string(aihj), "proxy", proxy)
		err = ioutil.WriteFile("aih.json", []byte(new_aihj), 0644)
		if err != nil {
			fmt.Println("Save failed.")
		}
		fmt.Println("Please restart Aih for using proxy...")
		Liner.Close()
		syscall.Exit(0)

	}

	// Set up client for OpenAI_API
	key := gjson.Get(string(aih_json), "key")
	OpenAI_Key := key.String()
	config := openai.DefaultConfig(OpenAI_Key)
	client := openai.NewClientWithConfig(config)
	messages := make([]openai.ChatCompletionMessage, 0)
	printer_chat := color.New(color.FgWhite)

	// Set up client for ChatGPT Web
	chat_access_token := gjson.Get(string(aih_json), "chat_access_token").String()
	var client_chat = &http.Client{}
	var conversation_id string
	var parent_id string

	// Set up client for GoogleBard
	bard_session_id := gjson.Get(string(aih_json), "__Secure-lPSID").String()
	bard_client := bard.NewBard(bard_session_id, "")
	bardOptions := bard.Options{
		ConversationID: "",
		ResponseID:     "",
		ChoiceID:       "",
	}
	printer_bard := color.New(color.FgRed).Add(color.Bold)

	// Set up client for BingChat
	var gpt *EdgeGPT.GPT
	_, err = ioutil.ReadFile("./cookies/1.json")
	if err == nil {
		s := EdgeGPT.NewStorage()
		gpt, err = s.GetOrSet("any-key")
		if err != nil {
			fmt.Println("Please reset bing cookie")
		}
	}
	printer_bing := color.New(color.FgCyan).Add(color.Bold)

	// Clean screen
	clear()

	// Welcome to Aih
	welcome := `
╭ ────────────────────────────── ╮
│    Welcome to Aih              │ 
│    Type .help for help         │ 
╰ ────────────────────────────── ╯ `
	fmt.Println(welcome)

	max_tokens := 4097
	used_tokens := 0
	left_tokens := 0
	speak := 0
	role := ".bard"
	uInput := ""

	// Start loop to read user input
	for {
		prompt := strconv.Itoa(left_tokens) + role + "> "
		userInput := multiln_input(Liner, prompt)

		// Check Aih commands
		switch userInput {
		case "":
			continue
		case ".proxy":
			proxy, _ := Liner.Prompt("Please input your proxy:")
			if proxy == "" {
				continue
			}
			aihj, err := ioutil.ReadFile("aih.json")
			new_aihj, _ := sjson.Set(string(aihj), "proxy", proxy)
			err = ioutil.WriteFile("aih.json", []byte(new_aihj), 0644)
			if err != nil {
				fmt.Println("Save failed.")
			}
			fmt.Println("Please restart Aih for using proxy")
			Liner.Close()
			syscall.Exit(0)
		case ".bardkey":
			bard_session_id = ""
			role = ".bard"
			goto BARD
			//continue
		case ".chatkey":
			chat_access_token = ""
			role = ".chat"
			goto CHART
			//continue
		case ".chatapikey":
			OpenAI_Key = ""
			role = ".chatapi"
			goto CHARTAPI
			//continue
		case ".bingkey":
			_ = os.Remove("./cookies/1.json")
			role = ".bing"
			goto BING
			//continue
		case ".help":
			fmt.Println(".bard        Google Bard")
			fmt.Println(".bing        Bing Chat")
			fmt.Println(".chat        ChatGPT Web (free)")
			fmt.Println(".chatapi     ChatGPT Api (pay)")
			fmt.Println(".proxy       Set proxy")
			fmt.Println("<<           Start multiple lines input")
			fmt.Println(">>           End multiple lines input")
			fmt.Println("↑            Previous input value")
			fmt.Println("↓            Next input value")
			fmt.Println(".new         New conversation of ChatGPT")
			fmt.Println(".speak       Voice speak context")
			fmt.Println(".quiet       Not speak")
			fmt.Println(".bardkey     Reset Google Bard cookie")
			fmt.Println(".bingkey     Reset Bing Chat coolie")
			fmt.Println(".chatkey     Reset ChatGPT Web accessToken")
			fmt.Println(".chatapikey  Reset ChatGPT Api key")
			fmt.Println(".clear or .c Clear screen")
			fmt.Println(".help        Help")
			fmt.Println(".exit        Exit")
			continue
		case ".speak":
			speak = 1
			continue
		case ".quiet":
			speak = 0
			continue
		case ".clear":
			clear()
			continue
		case ".c":
			clear()
			continue
		case ".exit":
			return
		case ".new":
			// For role .chat
			conversation_id = ""
			parent_id = ""
			// For role .chatapi
			messages = make([]openai.ChatCompletionMessage, 0)
			max_tokens = 4097
			used_tokens = 0
			left_tokens = max_tokens - used_tokens
			continue
		case ".bard":
			role = ".bard"
			left_tokens = 0
			continue
		case ".bing":
			role = ".bing"
			left_tokens = 0
			continue
		case ".chat":
			role = ".chat"
			left_tokens = 0
			continue
		case ".chatapi":
			role = ".chatapi"
			left_tokens = max_tokens - used_tokens
			continue
		case ".eng":
			role = ".eng"
			speak = 0
			left_tokens = 0
			continue
		default:
			// Record user input without Aih commands
			uInput = strings.Replace(userInput, "\r", "\n", -1)
			uInput = strings.Replace(uInput, "\n", " ", -1)
			Liner.AppendHistory(uInput)
			// Persistent
			if f, err := os.Create(".history"); err == nil {
				Liner.WriteHistory(f)
				f.Close()
			}

		}

		if role == ".eng" {
			userInput = "Please give me 30 single words in python list format that are relate to, opposite of, synonym of, description of, hyponymy or hypernymy of, part or wholes of, or rhythmic with the meaning of `" + userInput + "`"
			goto BARD
		}
	BARD:
		// Check role for correct actions
		if role == ".bard" || role == ".eng" {
			// Check GoogleBard session
			if bard_session_id == "" {
				bard_session_id, _ = Liner.Prompt("Please input your cookie value of __Secure-lPSID: ")
				if bard_session_id == "" {
					continue
				}
				aihj, err := ioutil.ReadFile("aih.json")
				nj, _ := sjson.Set(string(aihj), "__Secure-lPSID", bard_session_id)
				err = ioutil.WriteFile("aih.json", []byte(nj), 0644)
				if err != nil {
					fmt.Println("Save failed.")
				}
				// Renew GoogleBard client with __Secure-lPSID
				bard_client = bard.NewBard(bard_session_id, "")
				continue
			}

			// Send message
			response, err := bard_client.SendMessage(userInput, bardOptions)
			if err != nil {
				fmt.Println(err)
				continue
			}

			all_resp := response
			if all_resp != nil {
				RESP = response.Choices[0].Answer
				printer_bard.Println(RESP)
			} else {
				break
			}
			bardOptions.ConversationID = response.ConversationID
			bardOptions.ResponseID = response.ResponseID
			bardOptions.ChoiceID = response.Choices[0].ChoiceID
		}

	BING:
		if role == ".bing" {
			// Check BingChat cookie
			_, err := ioutil.ReadFile("./cookies/1.json")
			if err != nil {
				prom := "Please type << then paste Bing cookie then type >> then press Enter: "
				ck := multiln_input(Liner, prom)
				ck = strings.Replace(ck, "\r", "", -1)
				ck = strings.Replace(ck, "\n", "", -1)
				if len(ck) < 10 {
					continue
				}
				_ = os.MkdirAll("./cookies", 0755)
				err = ioutil.WriteFile("./cookies/1.json", []byte(ck), 0644)
				if err != nil {
					fmt.Println("Save failed.")
				}
				// Renew BingChat client with cookie
				s := EdgeGPT.NewStorage()
				gpt, err = s.GetOrSet("any-key")
				// Clear screen
				clear()
				continue
			}
			// Send message
			as, err := gpt.AskSync("creative", userInput)
			if err != nil {
				fmt.Println(err)
				continue
			}
			RESP = strings.TrimSpace(as.Answer.GetAnswer())
			printer_bing.Println(RESP)
		}

	CHART:
		if role == ".chat" {
			if chat_access_token == "" {
				chat_access_token, _ = Liner.Prompt("Please input your ChatGPT accessToken: ")
				if chat_access_token == "" {
					continue
				}
				aihj, err := ioutil.ReadFile("aih.json")
				nj, _ := sjson.Set(string(aihj), "chat_access_token", chat_access_token)
				err = ioutil.WriteFile("aih.json", []byte(nj), 0644)
				if err != nil {
					fmt.Println("Save failed.")
				}
				continue
			}

			// Send message
			RESP = chatgpt_web(client_chat, &chat_access_token, &userInput, &conversation_id, &parent_id)
			printer_chat.Println(RESP)

		}

	CHARTAPI:
		if role == ".chatapi" {
			// Check ChatGPT API Key
			if OpenAI_Key == "" {
				OpenAI_Key, _ = Liner.Prompt("Please input your OpenAI Key: ")
				if OpenAI_Key == "" {
					continue
				}
				aihj, err := ioutil.ReadFile("aih.json")
				new_aihj, _ := sjson.Set(string(aihj), "key", OpenAI_Key)
				err = ioutil.WriteFile("aih.json", []byte(new_aihj), 0644)
				if err != nil {
					fmt.Println("Save failed.")
				}
				// Renew ChatGPT client with key
				config = openai.DefaultConfig(OpenAI_Key)
				client = openai.NewClientWithConfig(config)
				messages = make([]openai.ChatCompletionMessage, 0)
				left_tokens = 0
				continue
			}
			// Porcess input
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: userInput,
			})

			// Generate a response from ChatGPT
			resp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:    openai.GPT3Dot5Turbo,
					Messages: messages,
				},
			)

			if err != nil {
				fmt.Println(err)
				continue
			}

			// Print the response to the terminal
			RESP = strings.TrimSpace(resp.Choices[0].Message.Content)
			used_tokens = resp.Usage.TotalTokens
			left_tokens = max_tokens - used_tokens
			printer_chat.Println(RESP)

			// Record in coversation context
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: RESP,
			})

		}

		// Write response RESP to clipboard
		err = clipboard.WriteAll(RESP)
		if err != nil {
			panic(err)
		}

		// Speak all the response RESP using the "say" command
		if speak == 1 {

			fmt.Println("speaking")
			go func() {
				switch runtime.GOOS {
				case "linux", "darwin":
					cmd := exec.Command("say", RESP)
					err = cmd.Run()
					if err != nil {
						fmt.Println(err)
					}
				case "windows":
					// to do
					_ = 1 + 1

				}

			}()
		}
		// Play video
		if role == ".eng" {

			re := regexp.MustCompile(`(?s)\[[^\[\]]*\]`)

			// Match the regular expression against the Python list.
			match := re.FindAllString(RESP, -1)

			if match != nil {
				lt_str := match[0]
				lt_str  = lt_str[1:len(lt_str)-1]
				lt_str  = strings.Replace(lt_str, `"`, "", -1)
				lt_str  = strings.Replace(lt_str, `'`, "", -1)
				ar := strings.Split(lt_str, ",")
			        go eng.Play(ar)
			}

		}

	}
}

func chatgpt_web(c *http.Client, chat_access_token, prompt, c_id, p_id *string) string {
	// Set the endpoint URL.
	var api = "https://ai.fakeopen.com/api"
	url := api + "/conversation"

	x := `{"action": "next", "messages": [{"id": null, "role": "user", "author": {"role": "user"}, "content": {"content_type": "text", "parts": [""]}}], 
                             "conversation_id": null, 
			     "parent_message_id": "", 
			     "model": "text-davinci-002-render-sha"}`

	x, _ = sjson.Set(x, "messages.0.content.parts.0", *prompt)

	m_id := uuid.New().String()
	x, _ = sjson.Set(x, "messages.0.id", m_id)

	if *p_id == "" {
		*p_id = uuid.New().String()
	}
	x, _ = sjson.Set(x, "parent_message_id", *p_id)

	if *c_id != "" {
		x, _ = sjson.Set(x, "conversation_id", *c_id)
	}

	// Create a new request.
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(x)))
	if err != nil {
		panic(err)
	}

	// Set the headers.
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *chat_access_token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Send the request.
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println(err, "service not work, please try again ...")
	}
	defer resp.Body.Close()

	// Check the response status code.
	if resp.StatusCode != 200 {
		panic(resp.Status)
	}

	// Read the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	// Find the whole response
	long_str := string(body)
	lines := strings.Split(long_str, "\n")
	long_str = lines[len(lines)-5]

	answer := gjson.Get(long_str[5:], "message.content.parts.0").String()
	*c_id = gjson.Get(long_str[5:], "conversation_id").String()
	*p_id = gjson.Get(long_str[5:], "message.id").String()
	return answer
}
