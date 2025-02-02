package hcat_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/dep"
	"github.com/hashicorp/hcat/tfunc"
)

// These examples requires a running consul to test against.
// For testing it is taken care of by TestMain.

const (
	exampleServiceTemplate = "{{range services}}{{range service .Name }}" +
		"service {{.Name }} at {{.Address}}" +
		"{{end}}{{end}}"
	exampleNodeTemplate = "{{range nodes}}node at {{.Address}}{{end}}"
	exampleKvTrigger    = `{{if keyExists "notify"}}{{end}}`
)

var examples = []string{exampleServiceTemplate, exampleNodeTemplate}

// Repeatedly runs the resolver on the template and watcher until the returned
// ResolveEvent shows the template has fetched all values and completed, then
// returns the output.
func RenderExampleOnce(clients *hcat.ClientSet) string {
	w := hcat.NewWatcher(hcat.WatcherInput{
		Clients: clients,
		Cache:   hcat.NewStore(),
	})
	tmpl := hcat.NewTemplate(hcat.TemplateInput{
		Contents:     exampleServiceTemplate,
		FuncMapMerge: tfunc.ConsulV0(),
	})
	w.Register(tmpl)

	ctx := context.Background()
	r := hcat.NewResolver()
	for {
		re, err := r.Run(tmpl, w)
		if err != nil {
			log.Fatal(err)
		}
		if re.Complete {
			return string(re.Contents)
		}
		// Wait pauses until new data has been received
		err = w.Wait(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// Runs the resolver over multiple templates until all have completed.
// By looping over all the templates it can start the data lookups in each and
// better share cached results for faster overall template rendering.
func RenderMultipleOnce(clients *hcat.ClientSet) string {
	w := hcat.NewWatcher(hcat.WatcherInput{
		Clients: clients,
		Cache:   hcat.NewStore(),
	})
	templates := make([]*hcat.Template, len(examples))
	for i, egs := range examples {
		tmpl := hcat.NewTemplate(
			hcat.TemplateInput{Contents: egs, FuncMapMerge: tfunc.ConsulV0()})
		templates[i] = tmpl
		w.Register(tmpl)
	}

	results := []string{}
	r := hcat.NewResolver()
	for {
		for i, tmpl := range templates {
			re, err := r.Run(tmpl, w)
			if err != nil {
				log.Fatal(err)
			}
			if re.Complete {
				results = append(results, string(re.Contents))
				templates = append(templates[:i], templates[(i+1):]...)
			}
		}
		if len(templates) == 0 {
			break
		}
		// Wait pauses until new data has been received
		ctx := context.Background()
		err := w.Wait(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	return strings.Join(results, ", ")
}

// An example of how you might implement a different notification strategy
// using a custom Notifier. In this case we are wrapping a standard template
// to have it only trigger notifications and be ready to be updated *only if*
// the KV value 'notify' is written to.
func NotifierExample(clients *hcat.ClientSet) string {
	w := hcat.NewWatcher(hcat.WatcherInput{
		Clients: clients,
		Cache:   hcat.NewStore(),
	})
	tmpl := KvNotifier{hcat.NewTemplate(hcat.TemplateInput{
		Contents:     exampleNodeTemplate + exampleKvTrigger,
		FuncMapMerge: tfunc.ConsulV0(),
	})}
	w.Register(tmpl)

	// post KV trigger after a brief pause
	// you'd probably do this via another means
	go func() {
		time.Sleep(time.Millisecond)
		_, err := clients.Consul().KV().Put(
			&api.KVPair{Key: "notify", Value: []byte("run")}, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	r := hcat.NewResolver()
	for {
		re, err := r.Run(tmpl, w)
		if err != nil {
			log.Fatal(err)
		}
		if re.Complete {
			return string(re.Contents)
		}
		// Wait pauses until new data has been received
		if err := w.Wait(context.Background()); err != nil {
			log.Fatal(err)
		}
	}
}

// Embed template to allow overridding of Notify
type KvNotifier struct {
	*hcat.Template
}

// Notify receives the updated value as the argument. You can then use the
// template function types published in ./dep/template_function_types.go to
// assert the types and notify based on the type and/or values.
// Note calling Template's Notify() is needed to mark it as having new data.
func (n KvNotifier) Notify(d interface{}) (notify bool) {
	switch d.(type) {
	case dep.KvValue:
		n.Template.Notify(d)
		return true
	default:
		return false
	}
}

// Shows multiple examples of usage from a high level perspective.
func Example() {
	if *hcat.RunExamples {
		clients := hcat.NewClientSet()
		defer clients.Stop()
		// consuladdr is set in TestMain
		clients.AddConsul(hcat.ConsulInput{Address: hcat.Consuladdr})

		fmt.Printf("RenderExampleOnce: %s\n",
			RenderExampleOnce(clients))
		fmt.Printf("RenderMultipleOnce: %s\n",
			RenderMultipleOnce(clients))
		fmt.Printf("NotifierExample: %s\n",
			NotifierExample(clients))
	} else {
		// so test doesn't fail when skipping
		fmt.Printf("RenderExampleOnce: %s\n", "service consul at 127.0.0.1")
		fmt.Printf("RenderMultipleOnce: %s\n",
			"node at 127.0.0.1, service consul at 127.0.0.1")
		fmt.Printf("NotifierExample: node at 127.0.0.1\n")
	}
	// Output:
	// RenderExampleOnce: service consul at 127.0.0.1
	// RenderMultipleOnce: node at 127.0.0.1, service consul at 127.0.0.1
	// NotifierExample: node at 127.0.0.1
}
