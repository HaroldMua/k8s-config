package main

import (
	"fmt"
	"time"
	"strings"

	glog "github.com/golang/glog"
	kubernetes "k8s.io/client-go/kubernetes"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	record "k8s.io/client-go/tools/record"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1 "k8s.io/api/core/v1"
	cache "k8s.io/client-go/tools/cache"
	workqueue "k8s.io/client-go/util/workqueue"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait "k8s.io/apimachinery/pkg/util/wait"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	patchtype "k8s.io/apimachinery/pkg/types"

	clientset "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/clientset/versioned"
	informers "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/informers/externalversions/tensorflow/v1"
	tensorflowscheme "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/clientset/versioned/scheme"
	listers "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/client/listers/tensorflow/v1"
	tensorflowv1 "lsalab.nthu.edu.tw/ericyeh/tensorflow-controller/pkg/apis/tensorflow/v1"
)

const controllerAgentName = "tensorflow-controller"

const patchjson = 
`{
	"spec": {
		"containers": [
			{ "name": "container1", "image": "REPLACE_IMAGE_NAME" }
		]
	}
}
`

const (
	SuccessSynced = "Synced"
	ErrResourceExists = "ErrResourceExists"

	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	MessageResourceSynced = "Foo synced successfully"
)

type Controller struct {
	kubeclientset kubernetes.Interface
	tensorflowclientset clientset.Interface

	podsLister corelisters.PodLister
	podsSynced cache.InformerSynced
	tensorflowLister listers.TensorflowLister
	tensorflowSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder record.EventRecorder
}

func NewController(
	kubeclientset kubernetes.Interface,
	tensorflowclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	tensorflowInformer informers.TensorflowInformer) *Controller {

	tensorflowscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset: kubeclientset,
		tensorflowclientset: tensorflowclientset,
		podsLister: podInformer.Lister(),
		podsSynced: podInformer.Informer().HasSynced,
		tensorflowLister: tensorflowInformer.Lister(),
		tensorflowSynced: tensorflowInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Tensorflows"),
		recorder: recorder,
	}

	glog.Info("Setting up event handlers")
	tensorflowInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTensorflow,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTensorflow(new)
		},
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Pod)
			oldDepl := old.(*corev1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

func (c *Controller) enqueueTensorflow(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "Tensorflow" {
			return
		}

		tensorflow, err := c.tensorflowLister.Tensorflows(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of tensorflow '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueTensorflow(tensorflow)
		return
	}
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	glog.Info("Starting Tensorflow controller")

	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.tensorflowSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	tensorflow, err := c.tensorflowLister.Tensorflows(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("tensorflow '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	PodName := tensorflow.Spec.Job
	if PodName == "" {
		runtime.HandleError(fmt.Errorf("%s: pod name must be specified", key))
		return nil
	}

	pod, err := c.podsLister.Pods(tensorflow.Namespace).Get(PodName)
	if errors.IsNotFound(err) {
		pod, err = c.kubeclientset.CoreV1().Pods(tensorflow.Namespace).Create(newPod(tensorflow))
	}

	if err != nil {
		return err
	}

	if !metav1.IsControlledBy(pod, tensorflow) {
		msg := fmt.Sprintf(MessageResourceExists, pod.Name)
		c.recorder.Event(tensorflow, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	if tensorflow.Spec.Image != "" && tensorflow.Spec.Image != pod.Spec.Containers[0].Image {
		glog.V(4).Infof("tensorflow %s image: %s, pod image: %s", name, tensorflow.Spec.Image, pod.Spec.Containers[0].Image)
		pod, err = c.kubeclientset.CoreV1().Pods(tensorflow.Namespace).Patch(PodName,
			patchtype.StrategicMergePatchType,
			[]byte(strings.Replace(patchjson, "REPLACE_IMAGE_NAME", tensorflow.Spec.Image, 1)))
	}

	if err != nil {
		return err
	}

	err = c.updateTensorflowStatus(tensorflow, pod)
	if err != nil {
		return err
	}

	c.recorder.Event(tensorflow, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

func (c *Controller) updateTensorflowStatus(tensorflow *tensorflowv1.Tensorflow, pod *corev1.Pod) error {
	tensorflowCopy := tensorflow.DeepCopy()
	tensorflowCopy.Status.CurrentImage = pod.Spec.Containers[0].Image
	_, err := c.tensorflowclientset.LsalabV1().Tensorflows(tensorflow.Namespace).Update(tensorflowCopy)
	return err
}

func newPod(tensorflow *tensorflowv1.Tensorflow) *corev1.Pod {
	labels := map[string]string{
		"app": "tensorflow",
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tensorflow.Spec.Job,
			Namespace: tensorflow.Namespace,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tensorflow, schema.GroupVersionKind{
					Group:   tensorflowv1.SchemeGroupVersion.Group,
					Version: tensorflowv1.SchemeGroupVersion.Version,
					Kind:    "Tensorflow",
				}),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
					Image: tensorflow.Spec.Image,
				},
			},
			RestartPolicy: "Never",
		},
	}
}
