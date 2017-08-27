package tf

import (
	"compress/gzip"
	"encoding/gob"
	"io"
	"strings"

	tensorflow "github.com/tensorflow/tensorflow/tensorflow/go"
)

// NumWeights returns the total number of weights the model has.
func (model *Model) NumWeights() (int64, error) {
	weights, err := model.WeightsMap()
	if err != nil {
		return 0, err
	}
	total := int64(0)
	for _, w := range weights {
		subtotal := int64(1)
		for _, dim := range w.Shape() {
			if dim >= 0 {
				subtotal *= dim
			}
		}
		total += subtotal
	}
	return total, nil
}

func (model *Model) trainableVariablesOutputs() ([]tensorflow.Output, error) {
	var outputs []tensorflow.Output
	for _, name := range model.Meta.TrainableVariables {
		op, n, err := ParseNodeOutput(name)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, model.Graph.Operation(model.ApplyPrefix(op)).Output(n))
	}
	return outputs, nil
}

func (model *Model) weights() ([]*tensorflow.Tensor, error) {
	outputs, err := model.trainableVariablesOutputs()
	if err != nil {
		return nil, err
	}
	results, err := model.Session.Run(
		nil,
		outputs,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (model *Model) WeightsMap() (map[string]*tensorflow.Tensor, error) {
	m := map[string]*tensorflow.Tensor{}

	weights, err := model.weights()
	if err != nil {
		return nil, err
	}

	for i, weight := range weights {
		key := model.Meta.TrainableVariables[i]
		m[key] = weight
	}

	return m, nil
}

func (model *Model) ExportWeights(wr io.Writer) error {
	weights, err := model.WeightsMap()
	if err != nil {
		return err
	}
	gzw := gzip.NewWriter(wr)
	defer gzw.Close()

	enc := gob.NewEncoder(gzw)
	if err := EncodeTensorMap(enc, weights); err != nil {
		return err
	}
	if err := gzw.Close(); err != nil {
		return err
	}
	return nil
}

// SetWeights sets the weights of the model.
func (model *Model) SetWeights(weights map[string]*tensorflow.Tensor) error {
	feeds := map[tensorflow.Output]*tensorflow.Tensor{}
	for _, variable := range model.Meta.TrainableVariables {
		opName := PokVarPrefix + strings.Replace(variable, ":", "/", -1)
		op := model.Graph.Operation(opName)
		feeds[op.Output(0)] = weights[variable]
	}

	if _, err := model.Session.Run(
		feeds,
		nil,
		[]*tensorflow.Operation{
			model.Graph.Operation(PokAssignOp),
		},
	); err != nil {
		return err
	}

	return nil
}

// AddWeights imports weights and then adds them to the current with a
// scaler.
func (model *Model) AddWeights(scale float64, weights map[string]*tensorflow.Tensor) error {
	scaleTensor, err := tensorflow.NewTensor(scale)
	if err != nil {
		return err
	}

	feeds := map[tensorflow.Output]*tensorflow.Tensor{
		model.Graph.Operation(PokVarScaleOp).Output(0): scaleTensor,
	}
	for _, variable := range model.Meta.TrainableVariables {
		opName := PokVarPrefix + strings.Replace(variable, ":", "/", -1)
		op := model.Graph.Operation(opName)
		feeds[op.Output(0)] = weights[variable]
	}

	if _, err := model.Session.Run(
		feeds,
		nil,
		[]*tensorflow.Operation{
			model.Graph.Operation(PokAssignAddOp),
		},
	); err != nil {
		return err
	}

	return nil
}

func LoadWeights(r io.Reader) (map[string]*tensorflow.Tensor, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	decoder := gob.NewDecoder(gzr)
	m, err := DecodeTensorMap(decoder)
	if err != nil {
		return nil, err
	}
	return m, nil
}

const (
	PokAssignOp    = "pok/update/assign"
	PokAssignAddOp = "pok/update/assign_add"
	PokVarPrefix   = "pok/update/var/"
	PokVarScaleOp  = "pok/update/scale"
)
