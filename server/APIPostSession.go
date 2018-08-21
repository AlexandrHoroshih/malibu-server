package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/graph-uk/combat-server/server/apireqresp"
)

func (t *CombatServer) createSessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionName := strconv.FormatInt(time.Now().UnixNano(), 10)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}

	reqStruct, err := apireqresp.ParseReqPostSessionFromBytes(body)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(reqStruct.SessionParams)

	//create dir, and save the file.
	os.MkdirAll("./sessions/"+sessionName, 0777)
	f, err := os.OpenFile("./sessions/"+sessionName+"/archived.zip", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	decodedFile, err := reqStruct.GetDecodedFile()
	if err != nil {
		fmt.Println(err)
		return
	}

	io.Copy(f, bytes.NewReader(decodedFile))

	//create session in DB.
	req, err := t.mdb.DB.DB().Prepare("INSERT INTO Sessions(id,params) VALUES(?,?)")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	_, err = req.Exec(sessionName, reqStruct.SessionParams)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//Mark all unfinished cases as finished and failed
	req, err = t.mdb.DB.DB().Prepare(`UPDATE Cases SET inProgress=false, passed=false, finished=true WHERE finished=false`)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	_, err = req.Exec()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	w.Header().Add(`Location`, sessionName)
	w.WriteHeader(http.StatusCreated)

	//io.WriteString(w, sessionName)
	fmt.Println(r.RemoteAddr + " Create new session: " + sessionName + " " + reqStruct.SessionParams)

	go t.doCasesExplore(reqStruct.SessionParams, sessionName)
}

func (t *CombatServer) doCasesExplore(params, sessionID string) error {
	err := unzip("./sessions/"+sessionID+"/archived.zip", "./sessions/"+sessionID+"/unarch")
	if err != nil {
		fmt.Print(err.Error())
		return err
	}
	os.Chdir("./sessions/" + sessionID + "/unarch/src/Tests")
	rootTestsPath, _ := os.Getwd()
	rootTestsPath += string(os.PathSeparator) + ".." + string(os.PathSeparator) + ".."

	command := []string{"cases"}
	for _, curParameter := range strings.Split(params, " ") {
		if strings.TrimSpace(curParameter) != "" {
			command = append(command, curParameter)
		}
	}
	cmd := exec.Command("combat", command...)
	cmd.Env = t.addToGOPath(rootTestsPath)

	var out bytes.Buffer
	var outErr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &outErr
	exitStatus := cmd.Run()

	os.Chdir(t.startPath)
	if exitStatus == nil {
		t.setCasesForSession(out.String(), sessionID)
	} else {

		req, err := t.mdb.DB.DB().Prepare("UPDATE Sessions SET casesExploringFailMessage=? WHERE id=?")
		if err != nil {
			fmt.Println(err)
			return err
		}
		_, err = req.Exec(out.String()+"\r\n"+outErr.String(), sessionID) // cases ExploringStarted
		if err != nil {
			fmt.Println(err)
			return err
		}

		fmt.Println("Cannot extract cases")
		fmt.Println(out.String())
		fmt.Println(outErr.String())
		return errors.New("Cannot extract combat cases.")
	}

	return nil
}

func (t *CombatServer) setCasesForSession(sessionCases, sessionID string) error {
	sessionCasesArr := strings.Split(sessionCases, "\n")

	req, err := t.mdb.DB.DB().Prepare("INSERT INTO Cases(cmdline, sessionID) VALUES(?,?)")
	if err != nil {
		fmt.Println(err)
		return (err)
	}

	casesCount := 0
	for _, curCase := range sessionCasesArr {
		curCaseCleared := strings.TrimSpace(curCase)
		if curCaseCleared != "" {
			casesCount++
			_, err = req.Exec(curCase, sessionID)
			if err != nil {
				fmt.Println(err)
				return (err)
			}
		}
	}

	fmt.Println("Explored " + strconv.Itoa(casesCount) + " cases for session: " + sessionID)

	go t.DeleteOldSessions()
	return nil
}