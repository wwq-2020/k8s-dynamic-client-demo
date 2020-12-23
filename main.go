package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	tpl  = flag.String("t", "", "-t=xx.tpl")
	data = flag.String("d", "", "-d=data")
)

func main() {
	flag.Parse()
	if *tpl == "" || *data == "" {
		flag.PrintDefaults()
		return
	}
	tplData, err := ioutil.ReadFile(*tpl)
	if err != nil {
		logrus.Fatal(err)
	}
	var dataData map[string]string
	if _, err := toml.DecodeFile(*data, &dataData); err != nil {
		logrus.Fatal(err)
	}
	t, err := template.New("").Parse(string(tplData))
	if err != nil {
		logrus.Fatal(err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, dataData); err != nil {
		logrus.Fatal(err)
	}

	yamls := bytes.Split(buf.Bytes(), []byte("---"))
	if len(yamls) == 0 {
		logrus.Info("no yaml")
		return
	}

	kubeConfigPath := os.ExpandEnv("$HOME/.kube/config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		logrus.Fatal(err)
	}

	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal(err)
	}

	resources, err := restmapper.GetAPIGroupResources(c.Discovery())
	if err != nil {
		logrus.Fatal(err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(resources)
	dynamicREST, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	for _, each := range yamls {
		runtimeObject, groupVersionAndKind, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(each, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		mapping, err := mapper.RESTMapping(groupVersionAndKind.GroupKind(), groupVersionAndKind.Version)
		if err != nil {
			logrus.Fatal(err)
		}

		unstructuredObj := runtimeObject.(*unstructured.Unstructured)
		var resourceREST dynamic.ResourceInterface = dynamicREST.Resource(mapping.Resource)
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace("default")
			}
			resourceREST = dynamicREST.
				Resource(mapping.Resource).
				Namespace(unstructuredObj.GetNamespace())
		}

		if _, err = resourceREST.Create(context.Background(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			logrus.Fatal(err)
		}
	}
}
