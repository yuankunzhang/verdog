package main

import (
    "fmt"
    "io/ioutil"
    "encoding/json"
    "net/http"
    "regexp"
    "sync"
    "os"
    "bufio"
    "strings"

    "github.com/deckarep/gosx-notifier"
    "os/exec"
)

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintf(os.Stderr, "Usage: %s check/add\n", os.Args[0])
        os.Exit(1)
    }

    switch os.Args[1] {
    case "check":
        Check()
    case "add":
        Add()
    default:
        fmt.Fprintf(os.Stderr, "Usage: %s check/add\n", os.Args[0])
        os.Exit(1)
    }
}

func Check() {
    fmt.Print(WelcomeMessage)
    fmt.Println("Checking library updates...")

    libs := ReadRegistry()

    var wg sync.WaitGroup
    wg.Add(len(libs))

    requireUpdate := false

    for i := range libs {
        go func(i int) {
            defer wg.Done()
            ver := GetSourceVersion(libs[i])
            if ver != libs[i].Version {
                requireUpdate = true
                UpdateAlert(libs[i], ver)
                libs[i].SetVersion(ver)

                if libs[i].Hook != "" {
                    ExecHook(libs[i].Hook, ver)
                }
            }
        }(i)
    }

    wg.Wait()

    if requireUpdate {
        SaveRegistry(libs)
    } else {
        fmt.Print("Nothing updated.")
    }
}

func Add() {
    fmt.Print(WelcomeMessage)
    fmt.Println("Adding new library, please fill in the following information:")

    reader := bufio.NewReader(os.Stdin)

    fmt.Print("Library name (e.g. prometheus): ")
    name, _ := reader.ReadString('\n')

    fmt.Print("Current version (e.g. 1.6.2): ")
    version, _ := reader.ReadString('\n')

    fmt.Print("URL: ")
    url, _ := reader.ReadString('\n')

    fmt.Print("Regex: ")
    regex, _ := reader.ReadString('\n')

    fmt.Print("Hook file (under the hooks/ folder): ")
    hook, _ := reader.ReadString('\n')

    lib := Library{
        Name: strings.TrimSpace(name),
        Version: strings.TrimSpace(version),
        Url: strings.TrimSpace(url),
        Regex: strings.TrimSpace(regex),
        Hook: strings.TrimSpace(hook),
    }

    libs := ReadRegistry()
    libs = append(libs, lib)
    SaveRegistry(libs)

    fmt.Printf("\nLibrary added:\n%#v", lib)
}

const (
    Version = "0.0.1"
    WelcomeMessage = "-- hello verdog v" + Version + " --\n\n"
    RegistryFilePath = "registry.json"
)

type Library struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Url         string `json:"url"`
    Regex       string `json:"regex"`
    Hook        string `json:"hook"`
}

func (lib *Library) SetVersion(version string) {
    lib.Version = version
}

func ReadRegistry() []Library {
    raw, err := ioutil.ReadFile(RegistryFilePath)
    if err != nil {
        panic(err)
    }

    var libs []Library
    json.Unmarshal(raw, &libs)
    return libs
}

func SaveRegistry(libs []Library) {
    bytes, err := json.MarshalIndent(libs, "", "    ")
    if err != nil {
        panic(err)
    }

    ioutil.WriteFile(RegistryFilePath, bytes, 0644)
}

func UpdateAlert(lib Library, newVersion string) {
    info := fmt.Sprintf("Library `%s` requires a version update: %s -> %s", lib.Name, lib.Version, newVersion)
    fmt.Println(info)

    notif := gosxnotifier.NewNotification(info)
    notif.Title = "Verdog"
    notif.Sound = gosxnotifier.Basso
    notif.Group = "me.yuankun.verdog"
    notif.AppIcon = "gopher.png"
    notif.Push()
}

func GetSourceVersion(lib Library) string {
    response, err := http.Get(lib.Url)
    if err != nil {
        panic(err)
    }

    defer response.Body.Close()

    body, err := ioutil.ReadAll(response.Body)
    if err != nil {
        panic(err)
    }

    r := regexp.MustCompile(lib.Regex)

    match := r.FindStringSubmatch(string(body))
    result := make(map[string]string)
    for i, name := range r.SubexpNames() {
        if i > 0 && i <= len(match) {
            result[name] = match[i]
        }
    }

    return result["Version"]
}

func ExecHook(hook, version string) {
    script := "./hooks/" + hook
    if _, err := os.Stat(script); err != nil {
        panic(err)
    }

    out, err := exec.Command(script, version).Output()
    if err != nil {
        panic(err)
    }

    fmt.Println(string(out))
}