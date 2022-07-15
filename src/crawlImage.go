package main

import (
	. "fmt"
	"log"
	"net/http"
	"os/exec"
	"pipeline"
	"runtime"
	"sync"

	"golang.org/x/net/html"
)

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

//검색한 이미지의 url 저장... slice여야할까..? 체널 하나에 값이 여러개 들어가도 되는가..?ㅇㅇ
type ImageResult struct {
	url string

	imageUrl string
}

//현재 페이지에 있는 포스트들 리스트. 체널에 값이 전송되는순간 트리거가 되기때문에 리스트아니어도 괜찮을지도..
type PostResult struct {
	url     string
	postUrl string
}

//이미 방문한 곳이면 안가도 됨..ㅇㅇ
type FetchedUrl struct {
	m          map[string]error //has a
	sync.Mutex                  //is a
}
type FetchedImageUrl struct {
	m map[string]struct{}
	sync.Mutex
}
type ImageSearch struct {
	fetchedUrl      *FetchedUrl
	fetchedImageUrl *FetchedImageUrl
	p               *pipeline.Pipeline
	result          chan ImageResult
	url             string
}
type PostSearch struct {
	fetchedUrl *FetchedUrl
	p          *pipeline.Pipeline
	image      *ImageSearch
	result     chan PostResult
	url        string
}

func (g *ImageSearch) Request(url string) {
	g.p.Request <- &ImageSearch{
		fetchedUrl:      g.fetchedUrl,
		fetchedImageUrl: g.fetchedImageUrl,
		p:               g.p,
		result:          g.result,
		url:             url,
	}
}
func (g *PostSearch) Request(url string) {
	g.p.Request <- &PostSearch{
		fetchedUrl: g.fetchedUrl,
		p:          g.p,
		image:      g.image,
		result:     g.result,
		url:        url,
	}
}

//긁어온정보를 결과에 보냄. 페이지의 url과 여러개의 이미지url, article이 나올것으로 기대함..
func (g *ImageSearch) Crawl() {
	g.fetchedUrl.Lock()
	if _, ok := g.fetchedUrl.m[g.url]; ok { //m[url]이 ok이라는것은 map[현재url]에 에러가 있다는것임. not nil이라는것. 이미 한번 방문한 url은  map에 에러를 넣어 저장했음
		g.fetchedUrl.Unlock()
		return
	}
	g.fetchedUrl.Unlock()
	doc, err := fetch(g.url)
	// Println("url ", g.url)
	// openbrowser(g.url)
	if err != nil {
		go func(u string) {
			g.Request(u)
		}(g.url)
		return
	}
	g.fetchedUrl.Lock()
	g.fetchedUrl.m[g.url] = err
	g.fetchedUrl.Unlock()

	imageUrls := g.Parse(doc)
	for r := range imageUrls {
		g.fetchedImageUrl.Lock()
		//fetched repo..?
		if _, ok := g.fetchedImageUrl.m[r]; !ok { //왜 not ok지? 에러가 없을경우인가
			g.result <- ImageResult{g.url, r}
			g.fetchedImageUrl.m[r] = struct{}{}
		}
		g.fetchedImageUrl.Unlock()
	}
	// g.result <- SearchResult{g.url, imageUrls}
}

//긁어온정보를 결과에 보냄. 페이지의 url과 여러개의 이미지url, article이 나올것으로 기대함..
func (g *PostSearch) Crawl() {
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

	postUrl := g.Parse(doc)
	for r := range postUrl {

		g.fetchedUrl.Lock()
		if _, ok := g.fetchedUrl.m[r]; !ok { //왜 not ok지? 에러가 없을경우인가
			g.result <- PostResult{g.url, r}
			g.fetchedUrl.m[r] = nil
		}
		g.fetchedUrl.Unlock()
	}
	// g.result <- PostResult{g.url, postUrl}
}

func (g *PostSearch) Parse(doc *html.Node) <-chan string { //receive 전용체널 받기전용. 체널로부터 받기라는거야.

	postUrl := make(chan string)
	count := 0
	// var urlList []string
	go func() {
		defer close(postUrl)
		var f func(*html.Node)
		f = func(n *html.Node) {

			if n.Type == html.ElementNode {
				// Println("node,", n.Type)
				switch n.Data {
				case "span":
					for _, a := range n.Attr {
						if a.Key == "class" && a.Val == "vcol col-id" {

							if n.Parent.Parent.Attr != nil {
								if len(n.Parent.Parent.Attr) != 2 {
									break
								}
								if n.Parent.Parent.Attr[0].Key == "class" && n.Parent.Parent.Attr[0].Val != "vrow notice notice-service" && n.Parent.Parent.Attr[0].Val != "vrow notice notice-board" && n.Parent.Parent.Attr[0].Val != "vrow notice notice-board filtered filtered-notice" {
									count++
									Println("parse count ", count, "https://arca.live"+n.Parent.Parent.Attr[1].Val)

									postUrl <- "https://arca.live" + n.Parent.Parent.Attr[1].Val

									g.Request("https://arca.live/b/genshin?p=6")

									g.image.Request("https://arca.live" + n.Parent.Parent.Attr[1].Val)
									break
								}
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

	return postUrl
}
func (g *ImageSearch) Parse(doc *html.Node) <-chan string { //receive 전용체널 받기전용. 체널로부터 받기라는거야.

	imageUrl := make(chan string)
	//count := 0
	// var urlList []string
	go func() {
		defer close(imageUrl)
		var f func(*html.Node)
		f = func(n *html.Node) {

			if n.Type == html.ElementNode {
				// Println("node,", n.Type)
				switch n.Data {

				case "div":
					//div 의 child를 찾음.. 예상기대 p들..
					for _, a := range n.Attr {
						if a.Key == "class" && a.Val == "fr-view article-content" { //게시글 내용
							for c := n.FirstChild; c != nil; c = c.NextSibling {

								if c.Data == "p" {
									//p 테그를 찾았으면 그것의 child를 찾음.. 예상기대 a..
									if c.FirstChild == nil {
										break
									}
									if atag := c.FirstChild; atag.Data == "img" {
										//Println("img url:", atag.Attr[0].Val)

										// urlList = append(urlList, atag.Attr[0].Val)
										imageUrl <- atag.Attr[0].Val
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

	image := &ImageSearch{
		fetchedUrl:      &FetchedUrl{m: make(map[string]error)},
		fetchedImageUrl: &FetchedImageUrl{m: make(map[string]struct{})},
		p:               p,
		result:          make(chan ImageResult),
		url:             url,
	}
	post := &PostSearch{
		fetchedUrl: &FetchedUrl{m: make(map[string]error)},
		p:          p,
		image:      image,
		result:     make(chan PostResult),
		url:        url,
	}
	p.Request <- post
	count := 0
	// LOOP:
	for {
		select {
		case f := <-image.result:
			//Println(f.url)
			//openbrowser(f.url)
			// for _, v := range f.imageUrl {
			// 	Println("\t\t", v)
			// }
			Println("\t\t", f.imageUrl)
			//			Println("count", count)
			if count == 30 {
				Println("종료됨")
				//	close(p.Done)
				// break LOOP
			}
			//count++
		case p := <-post.result:
			// Println(p.postUrl)
			openbrowser(p.postUrl)
			// for _, v := range p.postUrl {
			// 	Println("\t\t", v)
			// }
			// Println("post count", count)
			if count == 30 {
				Println("종료됨")
				//	close(p.Done)
				//break LOOP
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
		err = Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
