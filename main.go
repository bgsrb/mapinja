package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/headzoo/surf.v1"
)

//Company job Types
type JobType string

const (
	FullTimeJobType   JobType = "is_fulltime"
	InternshipJobType JobType = "internship"
	ParttimeJobType   JobType = "is_parttime"
	RemoteJobType     JobType = "remote"

	PORT = "8080"
)

//Company Model
type Company struct {
	URL      string
	Map      string
	Title    string
	Logo     string
	Location string
	Category string
	Hiring   bool
	Jobs     []Job
}

//Job Model
type Job struct {
	URL        string
	Title      string
	PassedDays string
	IsExpired  bool
	Type       JobType
}

const (
	//Const Base Path
	//CompaniesPath = "https://jobinja.ir/companies?page=%d"
	CompaniesPath   = "https://jobinja.ir/company/list/کامپیوتر-فناوری-اطلاعات-و-اینترنت?page=%d"
	CompanyPath     = "https://jobinja.ir/companies/%s"
	CompanyJobsPath = "https://jobinja.ir/companies/%s/jobs"
)

var companies []Company

func startCrawler() {

	// Create a new browser and open reddit.
	bow := surf.NewBrowser()
	bow.SetTimeout(time.Second * 5)

	//companies := make(map[string]Company)

	//start:
	companies = []Company{}

	for i := 1; ; i++ {
	retry:
		fmt.Println(i)
		err := bow.Open(fmt.Sprintf(CompaniesPath, i))
		if err != nil {
			log.Println(err)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				// 5 minute sleep for throttling
				time.Sleep(time.Minute * 5)
				fmt.Println("try")
				goto retry
			}
			continue
		}

		dom := bow.Find(".c-companyOverview")
		if dom.Length() == 0 {
			//Save Companies
			SaveCompanies(companies)
			fmt.Println(dom.Text())
			break
			//goto start
		}

		dom.Each(func(_ int, s *goquery.Selection) {

			//Find Company
			company, _ := FindCompany(s)

			//Find Company Jobs
			id := path.Base(company.URL)

			fmt.Println(company.URL)

			//if the company information is incomplete (map not exists)
			if id == "jobs" {
				return
			}

			jobs, _ := FindCompanyJobs(fmt.Sprintf(CompanyJobsPath, id))

			if len(jobs) == 0 {
				return
			}

			company.Jobs = jobs
			//Find Company Map
			company.Map = FindCompanyMap(fmt.Sprintf(CompanyPath, id))
			companies = append(companies, company)
		})
	}

}

//SaveCompanies save companies to file as json
func SaveCompanies(companies []Company) error {
	if len(companies) > 0 {
		jcompanies, _ := json.Marshal(companies)
		err := ioutil.WriteFile("./static/data.json", jcompanies, 0644)
		if err != nil {
			log.Println("WriteFile err : ", err)
		}
	}
	return nil
}

//FindCompany find and extract company entry form dom html
func FindCompany(s *goquery.Selection) (Company, error) {

	href, _ := s.Attr("href")
	meta := s.Find(".c-companyOverview__meta")
	title := meta.Find(".c-companyOverview__title").Text()
	logo, _ := meta.Find(".c-companyOverview__logo img.c-companyOverview__logoImage").Attr("src")

	tags := make(map[int]string)
	meta.Find(".c-companyOverview__tags span").Each(func(i int, s *goquery.Selection) {
		tags[i] = s.Text()
	})

	location, _ := tags[0]
	category, _ := tags[1]
	hiring, _ := tags[2]

	return Company{
		URL:      href,
		Title:    Clean(title),
		Logo:     logo,
		Location: location,
		Category: category,
		Hiring:   hiring == "در حال استخدام",
	}, nil
}

//FindCompanyMap find and extract company map form dom html
func FindCompanyMap(u string) string {
	bow := surf.NewBrowser()
	err := bow.Open(u)
	if err != nil {
		return ""
	}
	href, exists := bow.Find(".c-companyMap__mapLink").Attr("href")
	//fmt.Println("FindCompanyMap ", u, " ", href)
	if !exists {
		return ""
	}
	return strings.Split(strings.Split(href, "?")[1], "=")[1]
}

//FindCompanyJobs find and extract company jobs form dom html
func FindCompanyJobs(u string) ([]Job, error) {
	bow := surf.NewBrowser()
	jobs := []Job{}
	err := bow.Open(u)
	if err != nil {
		return jobs, err
	}
	dom := bow.Find(".o-listView__itemInfo")
	if dom.Length() == 0 {
		return jobs, nil
	}
	dom.Each(func(_ int, s *goquery.Selection) {
		job, _ := FindCompanyJob(s)
		if !job.IsExpired {
			jobs = append(jobs, job)
		}
	})
	return jobs, nil
}

//FindCompanyJob find and extract company jobs form dom html
func FindCompanyJob(s *goquery.Selection) (Job, error) {
	href, _ := s.Find("h3.c-jobListView__title  a.c-jobListView__titleLink").Attr("href")
	title := s.Find("h3.c-jobListView__title  a.c-jobListView__titleLink").Text()
	passedDays := s.Find("h3.c-jobListView__title  span.c-jobListView__passedDays").Text()

	metas := []string{}
	s.Find("ul.c-jobListView__meta li.c-jobListView__metaItem").Each(func(_ int, s *goquery.Selection) {
		metas = append(metas, s.Text())
	})

	return Job{
		URL:        href,
		Title:      Clean(title),
		PassedDays: Clean(passedDays),
		IsExpired:  IsExpired(Clean(passedDays)),
		//Type:       metas[2],
	}, nil
}

//Clean remove space and back salsh n (\n)
func Clean(s string) string {
	// space := regexp.MustCompile(`\s+`)
	// s = space.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	return s
}

//IsExpired check job is expired
func IsExpired(passedDays string) bool {
	return passedDays == "(منقضی شده)"
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	view("index.html", w, nil)
}

func view(view string, w io.Writer, data interface{}) {
	v := filepath.Join("./", filepath.Clean(view))
	tmpl, _ := template.ParseFiles(v)
	tmpl.Execute(w, data)
}
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "/static/images/favicon.ico")
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	go startCrawler()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/favicon.png", faviconHandler)
	fmt.Println("listen 8080")
	log.Fatal(http.ListenAndServe(":"+PORT, nil))
}
