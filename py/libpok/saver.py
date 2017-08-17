import tensorflow as tf
import tempfile
import json
import shutil
import tarfile
import os
from os import path

# Constants used for files inside the model archive.
SaverDefName   = "saver_def.pb"
GraphDefName   = "graph_def.pb"
SavedModelName = "saved_model"
TrainableVariablesName = "trainable_variables.json"

# save saves the session to the specified target file path. This creates a
# format that Pok can read and load.
def save(sess, target="model.tar.gz"):
    dir = tempfile.mkdtemp("pok_model_save_py")

    _add_assign_add()

    saver = tf.train.Saver(max_to_keep=1)

    tf.train.write_graph(sess.graph_def, dir, GraphDefName, as_text=False)

    trainable_variables = [v.name for v in tf.trainable_variables()]
    with open(path.join(dir, TrainableVariablesName), "w") as file:
        file.write(json.dumps(trainable_variables))

    saver.save(sess, path.join(dir, SavedModelName))
    saver_def = saver.as_saver_def().SerializeToString()
    with open(path.join(dir, SaverDefName), "wb") as file:
        file.write(saver_def)

    tar = tarfile.open(target, "w:gz")
    for file in os.listdir(dir):
        def keep_file_name(tarinfo):
            tarinfo.name = file
            return tarinfo
        tar.add(path.join(dir, file), filter=keep_file_name)
    tar.close()

    shutil.rmtree(dir)

def _add_assign_add():
    scale = tf.placeholder("double", name="pok/update/scale")
    typedScales = {
        scale.dtype: scale,
    }

    group_assign = []
    group_assign_add = []

    for v in tf.trainable_variables():
        name = "pok/update/var/"+v.name.replace(':','/')
        vin = tf.placeholder(v.dtype, v.shape, name=name)
        typedScale = typedScales.get(v.dtype)
        if typedScale is None:
            typedScale = tf.cast(scale, v.dtype)
            typedScales[v.dtype] = typedScale

        group_assign.append(v.assign(vin))
        group_assign_add.append(v.assign_add(vin * typedScale))

    tf.group(*group_assign, name="pok/update/assign")
    tf.group(*group_assign_add, name="pok/update/assign_add")