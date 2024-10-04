## Overview

DockerSlim is known for its ability to shrink container images reducing the container attack surface that can be exploited. It's used by developers, DevOps engineers and security engineers.  Even though it's popular and useful a lot of users struggle to use it successfully because they don't know enough about the available capabilities and they don't know enough about the containerized applications they are trying to secure.

We will introduce an AI-based interface in DockerSlim that will allow users to communicate the high level intent and outcome they need and the AI engine will figure out the details how to achieve it and what capabilities in DockerSlim to use.

We will also introduce advanced AI-powered security insights capabilities where the low level data DockerSlim produces will be presented, contextualized and explained in a meaningful way based on the high level intent and outcome users requested. We will add a set of specialized security research agents that will look for and analyze suspicious container image components and application dependencies.

## Questions / Problems

1. The standard CLI-based interface is hard to use because users don't know about all CLI flags and when they do know about them they don't know how to use them especially with their specific container images. Is it possible to create an AI-powered interface that wouldn't require deep knowledge of the DockerSlim capabilities and its parameters? 

2. The data produced by the DockerSlim commands expecially by the XRAY command is often low level and it needs to be interpreted to derive high level security insights. This requires additional domain knowledge in security and it also requires a deep understanding of the containerized applications, which many users don't have. Is it possible to autonomously analyze low level data produced by DockerSlim and produce higher level security insights?

3. The current security related data produced by the tool is very basic. Is it possible to build a set of specialized security research agents that can identify advanced security threats and insights in the target container images?

4. Successfully shrinking container images and reducing the attack surface in the target container images requires a significant amount of knowledge about the containerized application, which many users often don't have especially when they are not the ones who developed the containerized application (e.g., DevOps engineers). Is it possible to create an AI-powered interface that can extract the necessary context from the target containerized application, so users don't need to figure out that container context and provide it explicitly to DockerSlim?


## Planned Options, Techniques and Designs to Explore

* Different RAG designs
* Different agent memory designs
* Different types of tool design to be used by agents
* Different agent workflow designs
* In context learning
* Prompt engineering
* Fine-tuning

## Expected Result

The goal is to determine if it's possible to address the outlined problems and to implement a functional solution for each problem that's also reliable, practical, cost effective and intuitive to make it easy to secure containerized applications even for non-experts.
