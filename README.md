# Kubernetes Proxy NG

The Kubernetes Proxy NG a new design of kube-proxy aimed at

- allowing Kubernetes business logic to evolve with minimal to no impact on backend implementations,
- improving scalability,
- improving the ability of integrate 3rd party environments,
- being library-oriented to allow packaging logic at distributor's will,
- provide gRPC endpoints for lean integration, extensibility and observability.

The project will provide multiple components, with the core being the API watcher that will serve the global and node-specific sets of objects.

More context can be found in the project's [KEP](https://github.com/kubernetes/enhancements/issues/2104).

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack](http://slack.k8s.io/)
- [Mailing List](https://groups.google.com/forum/#!forum/kubernetes-dev)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
