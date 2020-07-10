package main

import (
	"fmt"
	"github.com/dutchcoders/goftp"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/yaml"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func checkEnv() bool {
	var cmd = exec.Command("which", "git")
	var err = cmd.Run()

	if err != nil {
		fmt.Printf("git find err. %s", err)
		return false
	}

	cmd = exec.Command("test", "-f", "./config.yaml")
	err = cmd.Run()

	if err != nil {
		fmt.Printf("config.yaml find err. %s", err)
		return false
	}

	config.AddDriver(yaml.Driver)

	err = config.LoadFiles("./config.yaml")
	if err != nil {
		fmt.Printf("read config.yaml err. %s", err)
		return false
	}

	err = config.BindStruct("", &appConfigs)
	if err != nil {
		fmt.Printf("bind config err. %s", err)
		return false
	}

	cmd = exec.Command("./initGit.sh", appConfigs.Git.Site, appConfigs.Git.Username, appConfigs.Git.Password)
	err = cmd.Run()

	if err != nil {
		fmt.Printf("git init err. %s", err)
		return false
	}

	for index, git := range appConfigs.Gits {
		gitPath := strings.Split(git.Git.Repo, "/")
		appConfigs.Gits[index].Git.PName =  strings.Split(gitPath[len(gitPath) - 1], ".")[0]

		if appConfigs.Gits[index].Git.PName == "" {
			fmt.Printf("parse git project name err like: http://abc/c.git")
			return false
		}

		git.Git.PName = appConfigs.Gits[index].Git.PName

		cmd = exec.Command("test", "-d", git.CloneDir)
		err = cmd.Run()

		if err != nil {
			fmt.Printf("git checkout dir is not exist: %s, will use current dir.\n", color.FgRed.Render(git.CloneDir))
			git.CloneDir = "./"
			appConfigs.Gits[index].CloneDir = "./"
		}

		if !strings.HasSuffix(git.CloneDir, "/") {
			git.CloneDir = git.CloneDir + "/"
			appConfigs.Gits[index].CloneDir = git.CloneDir
		}

		fmt.Printf("will checkout git repo: %s, branch: %s\n", color.FgGreen.Render(git.Git.Repo), color.FgGreen.Render(git.Git.Branch))

		appConfig = git

		if !gitPull() {
			return false
		}

		if git.Ftp.Use && !tryFtp(nil) {
			return false
		}
	}

	if appConfigs.Port == 0 {
		appConfigs.Port = 8080
	}

	return true
}

func tryFtp(cb func(*goftp.FTP) bool) bool {
	var err error
	var ftp *goftp.FTP

	if ftp, err = goftp.Connect(appConfig.Ftp.Server + ":21"); err != nil {
		fmt.Printf("ftp connect err. %s\n", err)
		return false
	}

	defer ftp.Close()

	fmt.Println("fto successfully connected !!")

	if err = ftp.Login(appConfig.Ftp.Username, appConfig.Ftp.Password); err != nil {
		fmt.Printf("ftp auth err. %s\n", err)
		return false
	}

	if cb != nil {
		return cb(ftp)
	}

	return true
}

func gitPull() bool {
	var cmd = exec.Command("test", "-d", appConfig.CloneDir + appConfig.Git.PName)
	var err = cmd.Run()

	if err != nil {
		cmd = exec.Command("sh", "-c", "cd " + appConfig.CloneDir + " && " + "git clone " + appConfig.Git.Repo + " -b " + appConfig.Git.Branch + " " + appConfig.Git.PName)
		err = cmd.Run()

		if err != nil {
			fmt.Printf("git clone err. %s", err, )
			return false
		}

		if appConfig.Git.User != "" {
			cmd = exec.Command("sh", "-c", "cd " + appConfig.CloneDir + " && " + "chown -R " + appConfig.Git.User + ":" + appConfig.Git.User + " " + appConfig.Git.PName)
			err = cmd.Run()

			if err != nil {
				fmt.Printf("chown repo to %s err. %s", err, appConfig.Git.User)
				return false
			}
		}

		if appConfig.Ftp.Use {
			tryFtp(func(ftp *goftp.FTP) bool{
				err := ftp.Upload(appConfig.CloneDir + appConfig.Git.PName)
				return err == nil
			})
		}
	}else {
		cmd = exec.Command("sh", "-c", "cd " + appConfig.CloneDir + appConfig.Git.PName + " && " + "git pull")
		err = cmd.Run()

		if err != nil {
			fmt.Printf("git pull err. %s", err)
			return false
		}
	}

	return true
}

func ftpMkdir(path string, ftp *goftp.FTP) {
	var paths = strings.Split(path, "/")
	var _p = ""
	for _, p := range paths {
		_p = _p + "/" + p
		err := ftp.Cwd(_p)

		if err != nil {
			ftp.Mkd(_p)
		}
	}
}

func  ftpDoPush(path string) {
	fmt.Printf("will push %s\n", path)

	tryFtp(func(ftp *goftp.FTP) bool{
		var file *os.File
		var err error
		if file, err = os.Open(appConfig.CloneDir + appConfig.Git.PName + "/" + path); err != nil {
			fmt.Println("open file fail: " + appConfig.CloneDir + appConfig.Git.PName + "/" + path)
			return false
		}
		defer file.Close()
		ftpMkdir(filepath.Dir("/" + path), ftp)
		if err := ftp.Stor("/" + path, file); err != nil {
			fmt.Println("upload file fail.")
			return false
		}
		return true
	})
}

func ftpDoDelete(path string) {
	fmt.Printf("will delete %s\n", path)

	tryFtp(func(ftp *goftp.FTP) bool{
		if err := ftp.Dele("/" + path); err != nil {
			fmt.Printf("ftp delete err. %s\n", err)
			return false
		}
		return true
	})
}

func StringsContains(array []string, val string) (index int) {
	index = -1
	for i := 0; i < len(array); i++ {
		if array[i] == val {
			index = i
			return
		}
	}

	return
}

func ftpPush(gitlab *gitlabWebHook) {
	var push []string
	var deleted []string

	for _, commit := range gitlab.Commits {
		for _, add := range append(append([]string{}, commit.Added...), commit.Modified...) {
			if index := StringsContains(deleted, add); index > -1 {
				deleted = append(deleted[:index], deleted[index+1:]...)
			}

			if index := StringsContains(push, add); index == -1 {
				push = append(push, add)
			}
		}

		for _, remove := range commit.Removed {
			if index := StringsContains(push, remove); index > -1 {
				push = append(push[:index], push[index+1:]...)
			}

			if index := StringsContains(deleted, remove); index == -1 {
				deleted = append(deleted, remove)
			}
		}
	}

	for _, add := range push {
		ftpDoPush(add)
	}

	for _, del := range deleted {
		ftpDoDelete(del)
	}
}

type gitlabWebHook struct {
	Project struct {
		GitHttpUrl string `json:"git_http_url"`
	}

	Ref string `json:"ref"`

	Commits [] struct {
		Id string
		Added []string
		Modified []string
		Removed []string
	}
}

type app struct {
	Git struct {
		Repo string
		Branch string
		PName string
		User string
	}


	Ftp struct {
		Server string
		Username string
		Password string
		Use bool
	}

	CloneDir string
}

var appConfig = app{}

var appConfigs = struct {
	Gits []app
	Port int
	Git struct {
		Site string
		Username string
		Password string
		Key string
	}
}{}

func checkRepo(hook *gitlabWebHook) bool {
	for _, git := range appConfigs.Gits {
		if git.Git.Repo == hook.Project.GitHttpUrl && "refs/heads/" + git.Git.Branch == hook.Ref {
			appConfig = git
			return true
		}
	}

	fmt.Printf("repo not in allow list: %s.\n", hook.Project.GitHttpUrl)
	return false
}

func main() {
	if !checkEnv() {
		return
	}

	r := gin.Default()

	r.GET("/metrics", func(c *gin.Context) {
		c.String(200, `# HELP server is running.
# TYPE commit_push_run_state gauge
commit_push_run_state 1`)
	})

	r.POST("/wechat/hook", func(c *gin.Context) {
		action := c.GetHeader("X-Gitlab-Event")
		token := c.GetHeader("X-Gitlab-Token")
		c.JSON(200, gin.H{})

		if action == "Push Hook" && token == appConfigs.Git.Key {
			info := &gitlabWebHook{}
			err := c.ShouldBind(&info)
			if err != nil {
				fmt.Println("参数有误")
				return
			}

			fmt.Printf("will due repo %s\n", info.Project.GitHttpUrl)

			if checkRepo(info) && gitPull() {
				if appConfig.Ftp.Use {
					ftpPush(info)
				}
			}
		}
	})
	r.Run(":" + strconv.Itoa(appConfigs.Port))
}