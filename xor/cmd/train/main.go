package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"time"

	"github.com/fumin/bangbang/util/borgletlib"
	"github.com/fumin/bangbang/util/dm"
	awawtf "github.com/fumin/bangbang/util/tensorflow"
	tfpb "github.com/fumin/bangbang/util/tensorflow/protos_all_go_proto"
	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	tf "github.com/tensorflow/tensorflow/tensorflow/go"
)

var (
	port      = flag.Int("port", 0, "server listening port")
	xprofPort = flag.Int("xprof_port", 0, "xprof listening port. For some reason the C++ flags are not parsed, and the workaround is to define it here.")

	xmXID = flag.Int("xm_xid", -1, "XManager experiment ID")
	xmWID = flag.Int("xm_wid", -1, "XManager work unit ID")

	_ = flag.Int("xm_rid", -1, "XManager run unit ID, not used by us. Set here just to avoid Borg treating this passed in flag as an error.")
	_ = flag.Bool("xm_borg_mode", false, "XManager flag, not used by us. Set here just to avoid Borg treating this passed in flag as an error.")
)

type ExperimentConfig struct {
	Owner string
	XID   int
	WID   int

	RandomSeed     int64
	DirPrefix      string
	CheckpointSecs int
	LogSteps       int
	TestSamples    int
}

type EnvConfig struct {
	BatchSize int
}

type Config struct {
	Experiment ExperimentConfig
	Env        EnvConfig
	Model      *json.RawMessage
}

func getConfig() (*Config, error) {
	if !borgletlib.RunByBorglet() {
		modelConfigStr := `{
      "fc": [8],
      "fc_nonlin": "tanh",
      "optimizer": "gradient_descent",
      "learning_rate": 0.001,
      "gradient_clipping": -1
    }`
		modelConfig := json.RawMessage(modelConfigStr)

		config := &Config{
			Experiment: ExperimentConfig{
				XID:            -1,
				WID:            -1,
				DirPrefix:      "/tmp/zzz/bangbang",
				CheckpointSecs: 1,
				LogSteps:       1000,
				TestSamples:    100,
			},
			Env: EnvConfig{
				BatchSize: 5,
			},
			Model: &modelConfig,
		}
		return config, nil
	}

	config := Config{}
	if err := dm.GetConfig(*xmXID, *xmWID, &config); err != nil {
		return nil, errors.Wrap(err, "dm.GetConfig")
	}
	config.Experiment.XID = *xmXID
	config.Experiment.WID = *xmWID

	b, err := json.Marshal(config)
	if err != nil {
		return nil, errors.Wrap(err, "json.Marshal")
	}
	log.Infof("Running with config: %s", string(b))
	return &config, nil
}

type Logger struct {
	bigtable *dm.BigTable
}

func NewLogger(config ExperimentConfig) (*Logger, error) {
	logger := &Logger{}

	if borgletlib.RunByBorglet() {
		bigtable, err := dm.NewBigTable(config.Owner, config.XID, config.WID)
		if err != nil {
			return nil, errors.Wrap(err, "dm.NewBigTable")
		}
		logger.bigtable = bigtable
	}

	return logger, nil
}

func (lg *Logger) Write(step int, val map[string]string) error {
	if lg.bigtable != nil {
		if err := lg.bigtable.Write(step, val); err != nil {
			return errors.Wrap(err, "bigtable.Write")
		}
	}
	log.Infof("step: %d, val: %+v", step, val)
	return nil
}

func createModel(checkpointDir string, config *json.RawMessage) (*awawtf.SavedModel, error) {
	// Configure our session.
	sessConf := &tfpb.ConfigProto{}
	sessConf.LogDevicePlacement = true
	sessConfBytes, err := proto.Marshal(sessConf)
	if err != nil {
		return nil, errors.Wrap(err, "proto.Marshal")
	}
	sessOpt := &tf.SessionOptions{}
	// sessOpt.Target = "local"
	sessOpt.Config = sessConfBytes

	// Load previously checkpointed model if possible.
	model, err := awawtf.LoadModel(checkpointDir, sessOpt)
	if err == nil {
		return model, nil
	}

	modelBin := "github.com/fumin/bangbang/xor/model.py"
	model, err = awawtf.CreateModel(modelBin, config, sessOpt)
	if err == nil {
		return model, nil
	}

	return nil, errors.Wrap(err, "awawtf.CreateModel")
}

type Env struct {
	batchSize int
	inputsPH  tf.Output
	labelsPH  tf.Output
	xorInputs [][]float32
	xorLabels [][]float32
}

func NewEnv(model *awawtf.SavedModel, config EnvConfig) *Env {
	g := model.Model.Graph
	env := &Env{}
	env.batchSize = config.BatchSize
	env.inputsPH = g.Operation("inputs").Output(0)
	env.labelsPH = g.Operation("labels").Output(0)
	env.xorInputs = [][]float32{
		{1, 1},
		{0, 1},
		{1, 0},
		{0, 0},
	}
	env.xorLabels = [][]float32{
		{0},
		{1},
		{1},
		{0},
	}
	return env
}

func (env *Env) Feeds() (map[tf.Output]*tf.Tensor, error) {
	inputs := make([][]float32, env.batchSize)
	labels := make([][]float32, env.batchSize)
	for b := 0; b < env.batchSize; b++ {
		inputs[b] = make([]float32, 2)
		labels[b] = make([]float32, 1)
	}

	for b := 0; b < env.batchSize; b++ {
		r := rand.Intn(len(env.xorInputs))
		inputs[b] = env.xorInputs[r]
		labels[b] = env.xorLabels[r]
	}

	inputsTF, err := tf.NewTensor(inputs)
	if err != nil {
		return nil, errors.Wrap(err, "tf.NEwTensor")
	}
	labelsTF, err := tf.NewTensor(labels)
	if err != nil {
		return nil, errors.Wrap(err, "tf.NewTensor")
	}
	feeds := make(map[tf.Output]*tf.Tensor)
	feeds[env.inputsPH] = inputsTF
	feeds[env.labelsPH] = labelsTF
	return feeds, nil
}

type TFFetches struct {
	fetches []tf.Output
	targets []*tf.Operation
}

type Agent struct {
	model        *tf.SavedModel
	trainFetches *TFFetches
}

func NewAgent(model *awawtf.SavedModel) *Agent {
	g := model.Model.Graph
	trainFetches := &TFFetches{}
	fetches := make([]tf.Output, 0)
	fetches = append(fetches, g.Operation("step_incr").Output(0))
	fetches = append(fetches, g.Operation("Model/loss").Output(0))
	fetches = append(fetches, g.Operation("Model/pred").Output(0))
	trainFetches.fetches = fetches

	targets := make([]*tf.Operation, 0)
	targets = append(targets, g.Operation("Model/optimize"))
	trainFetches.targets = targets

	agent := &Agent{}
	agent.model = model.Model
	agent.trainFetches = trainFetches
	return agent
}

type TrainOutput struct {
	step int
	loss float32
	pred [][]float32
}

func (ag *Agent) Train(feeds map[tf.Output]*tf.Tensor) (*TrainOutput, error) {
	runRes, err := ag.model.Session.Run(
		feeds, ag.trainFetches.fetches, ag.trainFetches.targets)
	if err != nil {
		return nil, errors.Wrap(err, "sess.Run")
	}

	output := &TrainOutput{}
	output.step = int(runRes[0].Value().(int64))
	output.loss = runRes[1].Value().(float32)
	output.pred = runRes[2].Value().([][]float32)
	return output, nil
}

func main() {
	// Parse flags.
	if !borgletlib.RunByBorglet() {
		flag.Set("logtostderr", "true")
	}
	flag.Parse()

	// Start server to support profiling.
	go func() {
		addr := fmt.Sprintf("localhost:%d", *port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("%+v", err)
		}
	}()

	// Parse config.
	config, err := getConfig()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	expConf := config.Experiment

	// Setup experiment.
	rand.Seed(expConf.RandomSeed)
	expDir := filepath.Join(
		expConf.DirPrefix,
		fmt.Sprintf("%d", expConf.XID),
		fmt.Sprintf("%d", expConf.WID))

	// Create tensorflow model.
	checkpointDir := filepath.Join(expDir, "checkpoint")
	model, err := createModel(checkpointDir, config.Model)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	go func() {
		for {
			<-time.After(time.Duration(expConf.CheckpointSecs) * time.Second)
			awawtf.SaveModel(model, checkpointDir)
		}
	}()

	env := NewEnv(model, config.Env)
	agent := NewAgent(model)

	logger, err := NewLogger(expConf)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	for {
		feeds, err := env.Feeds()
		if err != nil {
			log.Fatalf("%+v", err)
		}
		output, err := agent.Train(feeds)
		if err != nil {
			log.Fatalf("%+v", err)
		}

		if (output.step % expConf.LogSteps) == 0 {
			val := make(map[string]string)
			val["loss"] = fmt.Sprintf("%f", output.loss)
			if err := logger.Write(output.step, val); err != nil {
				log.Fatalf("%+v", err)
			}

			log.Infof("inputs: %+v", feeds[env.inputsPH].Value().([][]float32))
			log.Infof("labels: %+v", feeds[env.labelsPH].Value().([][]float32))
			log.Infof("pred: %+v", output.pred)
		}
	}
}
