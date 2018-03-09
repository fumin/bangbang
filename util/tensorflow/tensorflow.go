package tensorflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fumin/bangbang/util/file"
	"github.com/fumin/bangbang/util/runfiles"
	tfpb "github.com/fumin/bangbang/util/tensorflow/protos_all_go_proto"
	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	tf "github.com/tensorflow/tensorflow/tensorflow/go"
)

const (
	// https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/saved_model/tag_constants.py#L32
	savedModelTag = "train"
	// https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/saved_model/constants.py#L51
	savedModelFilename = "saved_model.pb"
)

type SavedModel struct {
	Model *tf.SavedModel
	Proto *tfpb.SavedModel
}

func CreateModel(modelBin string, config *json.RawMessage, sessOpt *tf.SessionOptions) (*SavedModel, error) {
	modelConf := struct {
		Model     *json.RawMessage `json:"model"`
		ExportDir string           `json:"export_dir"`
		Tags      []string         `json:"tags"`
	}{
		Model:     config,
		ExportDir: "/tmp/model",
		Tags:      []string{savedModelTag},
	}
	modelConfJS, err := json.Marshal(modelConf)
	if err != nil {
		return nil, errors.Wrap(err, "json.Marshal")
	}

	modelBinFull := runfiles.Path(modelBin)
	configFlag := fmt.Sprintf("--config=%s", string(modelConfJS))
	binArgs := []string{"--logtostderr", configFlag}
	cmd := exec.Command(modelBinFull, binArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Infof("running command: %s %+v", modelBinFull, binArgs)
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "cmd.Run")
	}

	model, err := LoadModel(modelConf.ExportDir, sessOpt)
	if err != nil {
		return nil, errors.Wrap(err, "loadModel")
	}
	return model, nil
}

func LoadModel(modelDir string, sessOpt *tf.SessionOptions) (*SavedModel, error) {
	tfModel, err := tf.LoadSavedModel(modelDir, []string{savedModelTag}, sessOpt)
	if err != nil {
		return nil, errors.Wrap(err, "tf.LoadSavedModel")
	}

	smPBFName := filepath.Join(modelDir, savedModelFilename)
	protoBytes, err := file.ReadFile(context.Background(), smPBFName)
	smPB := tfpb.SavedModel{}
	if err := proto.Unmarshal(protoBytes, &smPB); err != nil {
		return nil, errors.Wrap(err, "proto.Unmarshal")
	}

	model := &SavedModel{
		Model: tfModel,
		Proto: &smPB,
	}
	return model, nil
}

func SaveModel(model *SavedModel, modelDir string) error {
	// Create folders.
	if err := file.DeleteAll(context.Background(), modelDir); err != nil {
		return errors.Wrap(err, "file.DeleteAll")
	}
	// https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/saved_model/constants.py#L61
	varDirName := "variables"
	varDir := filepath.Join(modelDir, varDirName)
	if err := file.MkdirAll(context.Background(), varDir, nil); err != nil {
		return errors.Wrap(err, "file.MkdirAll")
	}

	// Save the MetaGraphDef using tf.Saver
	feeds := make(map[tf.Output]*tf.Tensor)
	filenameConst := model.Model.Graph.Operation("save/Const").Output(0)
	// https://github.com/tensorflow/tensorflow/blob/master/tensorflow/python/saved_model/constants.py#L66
	varFName := "variables"
	saverFName := filepath.Join(varDir, varFName)
	filenameTS, err := tf.NewTensor(saverFName)
	if err != nil {
		return errors.Wrap(err, "tf.NewTensor")
	}
	feeds[filenameConst] = filenameTS
	saveOp := model.Model.Graph.Operation("save/Identity")
	targets := []*tf.Operation{saveOp}
	_, err = model.Model.Session.Run(feeds, nil, targets)
	if err != nil {
		return errors.Wrap(err, "session.Run")
	}

	smpb := model.Proto
	smpbBytes, err := proto.Marshal(smpb)
	if err != nil {
		return errors.Wrap(err, "proto.Marshal")
	}
	smFName := filepath.Join(modelDir, savedModelFilename)
	if err := file.WriteFile(context.Background(), smFName, smpbBytes); err != nil {
		return errors.Wrap(err, "file.WriteFile")
	}
	return nil
}
