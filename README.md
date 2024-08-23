# Force K8S node scaledown

Tool that runs in a loop, checks nodes for relevant pods and deletes the node if no relevant pod is running.

**Use at your own risk! This tool has severe potential of just nuking your cluster if it misbehaves. You have been warned!**

## Why?
Especially GPU nodes, tend to never scale down in GKE, which becomes QUITE expensive for not bringing any value. 
This tool checks if any relevant pod is (still) running on a node and deletes the node if not. 

## How to use it?
Easiest if, if you just use the prebuild docker image and deploy it as deployment in your k8s cluster.
In order to do this. Apply the deployment.yml from the repository to your cluster. It will deploy the service into the default namespace.

**Keep in mind: If you don't taint your nodes, the tool itself might run on a GPU node, which will prevent scaling down/deleting the node.**

```bash
kubectl apply -f deployment.yml
```
## Contributing
If you want to contribute, feel free to open a PR or report an issue. 
As this is a free side project: Please don't expect any timely response. 

**If you urgently need changes on this tool, or want to hire me for consulting, please contact me via [simon-frey.com](https://simon-frey.com/).**

## License

MIT - See [LICENSE.md](LICENSE.md) for more information.