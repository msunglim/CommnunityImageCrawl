package main

import (
	"fmt"
	. "fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	"pipeline"

	"golang.org/x/net/html"
)

func a(url string) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	// defer resp.Body.Close()
	// _, err = ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Fatalln(err)
	// 	return nil, err
	// }

	// log.Println(string(body))
	return resp, nil

}

//현재 url을 html.Node 포인터로 리턴해줌. 그 Node로 .Data, Type 이런걸 추출 할 수 있음.
func fetch(url string) (*html.Node, error) {
	res, err := http.Get(url)
	// res, err := a(url)
	// defer res.Body.Close()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	doc, err := html.Parse(res.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return doc, nil
}

//검색한 이미지의 url과 그에 관련된 기사제목을 저장... slice여야할까..? 체널 하나에 값이 여러개 들어가도 되는가..?
type SearchResult struct {
	url string

	imageUrl []string
}

//이미 방문한 곳이면 안가도 됨..ㅇㅇ
type FetchedUrl struct {
	m          map[string]error //has a
	sync.Mutex                  //is a
}

type GoogleImageSearch struct {
	fetchedUrl *FetchedUrl
	p          *pipeline.Pipeline
	result     chan SearchResult
	url        string
}

func (g *GoogleImageSearch) Request(url string) {
	g.p.Request <- &GoogleImageSearch{
		fetchedUrl: g.fetchedUrl,
		p:          g.p,
		result:     g.result,
		url:        url,
	}
}

//긁어온정보를 결과에 보냄. 페이지의 url과 여러개의 이미지url, article이 나올것으로 기대함..
func (g *GoogleImageSearch) Crawl() {
	g.fetchedUrl.Lock()
	if _, ok := g.fetchedUrl.m[g.url]; ok { //m[url]이 ok이라는것은 map[현재url]에 에러가 있다는것임. not nil이라는것. 이미 한번 방문한 url은  map에 에러를 넣어 저장했음
		g.fetchedUrl.Unlock()
		return
	}
	g.fetchedUrl.Unlock()
	doc, err := fetch(g.url)
	if err != nil {
		go func(u string) {
			g.Request(u)
		}(g.url)
		return
	}
	g.fetchedUrl.Lock()
	g.fetchedUrl.m[g.url] = err
	g.fetchedUrl.Unlock()

	urls := <-g.Parse(doc)
	g.result <- SearchResult{g.url, urls}
}

func (g *GoogleImageSearch) Parse(doc *html.Node) <-chan []string { //receive 전용체널 받기전용. 체널로부터 받기라는거야.

	imageUrl := make(chan []string)
	var urlList []string
	go func() {

		var f func(*html.Node)
		f = func(n *html.Node) {

			if n.Type == html.ElementNode {
				// Println("node,", n.Type)
				switch n.Data {
				case "span":
					for _, a := range n.Attr {
						if a.Key == "class" && a.Val == "vcol col-id" {
							for _, b := range n.Parent.Parent.Attr {
								// count := 0
								// for c := n.Parent.Parent.Parent.FirstChild; c != nil; c = c.NextSibling {
								// 	if c.Data == "a" {
								// 		count++
								// 	}
								// }
								// Println("len", count)

								if b.Key == "class" && b.Val == "vrow notice notice-service" { //공지사항ㅋㅋ. 잘못들어온거임 빠르게나가!
									break
								}
								if b.Key == "href" {
									// Println("new url:", "https://arca.live"+b.Val)
									g.Request("https://arca.live" + b.Val)
									// openbrowser("https://arca.live" + b.Val) 판도라의 상자임..
									break
								}
							}

						}

					}
				case "div":
					//div 의 child를 찾음.. 예상기대 p들..
					for _, a := range n.Attr {
						if a.Key == "class" && a.Val == "fr-view article-content" { //게시글 내용
							for c := n.FirstChild; c != nil; c = c.NextSibling {

								if c.Data == "p" {
									//p 테그를 찾았으면 그것의 child를 찾음.. 예상기대 a..
									if atag := c.FirstChild; atag.Data == "img" {
										//Println("img url:", atag.Attr[0].Val)

										urlList = append(urlList, atag.Attr[0].Val)
										//원인을 알아넴.. imageUrl은 string체널인데
										//한 페이지엔 하나의 imageurl만 리턴할 수 있던거야.
										//하지만 나는 여러개를 했고 그결과 하나의 imageurl에
										//여러 string이 쌓여서 뭔가 오류가 생겨버린거지ㅣ..
										//그래서 해결법은? imageurl체널을 string[]으로 바꿔서
										//여러개의 imageurl들을 한페이지로 부터 받을 수 있게하자.
										//break
									}
								}
							}
							if len(urlList) > 0 {
								imageUrl <- urlList
								break
							}
						}
					}

				}

			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc)
	}()
	return imageUrl
}
func main() {
	//cpu를 모두써서 빠르게 처리하자!
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	var url string
	Println("Please type the url of a website that you want to crawl.")
	Scan(&url)
	p := pipeline.NewPipeline()
	p.Run()

	search := &GoogleImageSearch{
		fetchedUrl: &FetchedUrl{m: make(map[string]error)},
		p:          p,
		result:     make(chan SearchResult),
		url:        url,
	}
	p.Request <- search
	count := 0
LOOP:
	for {
		select {
		case f := <-search.result:
			Println(f.url)
			//openbrowser(f.url)
			for _, v := range f.imageUrl {
				Println("\t\t", v)

			}
			Println("count", count)
			if count == 5000000 {
				close(p.Done)
				Println("종료됨")
				break LOOP
			}
			count++
		}
	}
}
func openbrowser(url string) {
	if len(url) == 0 {
		return
	}
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
