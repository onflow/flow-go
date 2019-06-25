# Contribution Guide

This guide provides a comprehensive overview of our development process, guidelines, and steps required to contribute to the project.

## Work Streams

Bamboo development is divided across several streams of work with the goal of separating concerns and facilitating rapid development. 

Each stream is owned by a Bamboo core team member who oversees and directs all development within that stream. As a contributor, you will be communicate with the owner of the stream in which you were assigned.

Stream owners will assign tasks to contributors and ensure that all TODOs are tracked.

| Stream         | Owner(s)                    | Packages  |
| -------------- | --------------------------- | --------- |
| Pre-execution  | Peter Siemens (@psiemens)   | [/internal/access](/internal/access), [/cmd/access](/cmd/access) |
| Execution      | Bastian Müller (@turbolent) | [/runtime](/runtime), [/language](/language), [/internal/execute](/internal/execute), [/cmd/execute](/cmd/execute) |
| Post-execution | Moar Zamski (@pazams)     | [/internal/access](/internal/access), [/cmd/access](/cmd/access) |
| Consensus | Alexander Hentschel (@AlexHentschel)     | [/consensus](/consensus), [/internal/security](/internal/security) |
| Networking | Yahya Hassanzadeh (@yhassanzadeh)     | [/network](/network) |
| Keystone | Tarak Ben Youssef (@tarakby)     | [/pkg/crypto](/pkg/crypto) |
| Emulator | Brian Ho (@mrbrianhobo)     | [/emulator](/emulator), [/sdk](/sdk)|
| Engineer Performance | Timofey Smirnov (@tsmirnov) | |

## Workflow
