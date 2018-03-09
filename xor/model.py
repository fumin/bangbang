#!/Users/awaw/me/my_virtualenv/tensorflow/bin/python2.7
"""Code."""

import json
import subprocess
import sonnet as snt
import tensorflow as tf

tf.flags.DEFINE_string("config", "", "config for this binary")


def _nonlin(nonlin_name):
  if nonlin_name == "tanh":
    nonlin = tf.tanh
  elif nonlin_name == "relu":
    nonlin = tf.nn.relu
  else:
    raise ValueError("unknown non-linearity {}".format(nonlin_name))
  return nonlin


class Model(snt.AbstractModule):
  """Model."""

  def __init__(self, config, name="Model"):
    super(Model, self).__init__(name=name)
    self._config = config

  def _build(self, inputs, labels):
    pred_shape = [1]
    output_sizes = self._config["fc"] + pred_shape
    mlp = snt.nets.MLP(
        output_sizes=output_sizes,
        activation=_nonlin(self._config["fc_nonlin"]))
    inputs = mlp(inputs)

    # Compute the predictions and loss.
    pred = tf.sigmoid(inputs, name="pred")
    loss_batched = tf.squeeze(tf.square(pred - labels), axis=1)
    loss = tf.reduce_mean(loss_batched, axis=0, name="loss")
    tf.identity(loss)  # Just to create dummy output.

    # Optimize for the loss.
    optimize_op = self._optimize(loss, "optimize")  # pylint: disable=unused-variable

  def _optimize(self, loss, name):
    learning_rate = self._config["learning_rate"]
    optimizer_name = self._config["optimizer"]
    if optimizer_name == "gradient_descent":
      optimizer = tf.train.GradientDescentOptimizer(learning_rate)
    elif optimizer_name == "momentum":
      optimizer = tf.train.MomentumOptimizer(learning_rate, 0.9)
    elif optimizer_name == "rmsprop":
      optimizer = tf.train.RMSPropOptimizer(learning_rate, momentum=0.9)
    grads_and_vars = optimizer.compute_gradients(loss)
    clip = self._config["gradient_clipping"]
    if clip > 0:
      grads_and_vars = [
          (tf.clip_by_value(gv[0], -clip, clip), gv[1])
          for gv in grads_and_vars]
    optimize_op = optimizer.apply_gradients(grads_and_vars, name=name)
    return optimize_op


def main(unused_argv=()):
  config = json.loads(tf.flags.FLAGS.config)

  step = tf.get_variable(
      "step", shape=(), dtype=tf.int64,
      initializer=tf.constant_initializer(0))
  step_incr = tf.assign_add(step, 1, name="step_incr")  # pylint: disable=unused-variable

  inputs = tf.placeholder(name="inputs", shape=(None, 2), dtype=tf.float32)
  labels = tf.placeholder(name="labels", shape=(None, 1), dtype=tf.float32)
  model = Model(config["model"])
  model(inputs, labels)  # pylint: disable=not-callable

  init_op = tf.global_variables_initializer()

  export_dir = config["export_dir"]
  subprocess.call(["rm", "-r", export_dir])
  builder = tf.saved_model.builder.SavedModelBuilder(export_dir)
  with tf.Session() as sess:
    sess.run(init_op)

    tags = config["tags"]
    tf.logging.info("export_dir %s, tag %s", export_dir, tags)
    builder.add_meta_graph_and_variables(sess, tags)
  builder.save()


if __name__ == "__main__":
  main()
