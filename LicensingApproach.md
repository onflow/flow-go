# Flow's Approach to Licensing

The founding team for Flow suggests that all open-source software built on or for Flow should adhere to the "[Coherent Open Source Licensing](https://licenseuse.org)" philosophy.
To that end, we encourage the use of one of the following three licenses, as applicable by the use case (order from most permissive, to most restrictive):

- Apache 2.0
- LGPL 3
- Affero GPL 3

These licenses are all approved by both the Free Software Foundation and the Open Source Initiative and contain clauses regarding software patenting to avoid hidden patent concerns. Additionally, they are all compatible with each other meaning there is no conflict using them together in any project (provided the project as a whole meets the requirements of the most restrictive of the licenses included).


### Apache 2.0

Apache 2.0 is the most permissive, and allows reuse - with customization - in proprietary software. We recommend that this should be the default for any code that has significant potential use for off-chain tools and/or applications. In particular, all sample code (including smart contracts), SDKs, utility libraries and tools should default to Apache 2.0.


### LGPL 3

LGPL 3 allows use in proprietary software, provided that any customizations to the licensed code are shared. We considered LGPL for Cadence to avoid a proprietary fork of the language, but decided to use the more permissive Apache 2.0 license to emphasize that we hope Cadence can be adopted outside of Flow. LGPL can be used for other parts of Flow where necessary (e.g. if the component is itself a modification of other LGPL-licensed code).


### Affero GPL 3

Affero GPL 3 (AGPL) should be used for all code that exists primarily to power node software. In particular, the core consensus and computation layers should be provided via AGPL where possible. We chose this license because we interpret the clauses in the AGPL regarding "public use [...] on a publicly accessible server" as covering the operation of a node in the public Flow network. By requiring all modifications to the core node software to be made publicly available, we increase the security of the network (allowing any custom modifications to undergo public scrutiny), while also ensuring that improvements to the core infrastructure benefit all participants in the network.


### Using the Unlicense

For small snippets of sample code, we can provide access via the Unlicense.
Broadly speaking, Apache 2.0 gives essentially the same rights to the receiver of the code as Unlicense, except that Apache protects the author(s) a bit more robustly, and protects the receiver against patent lawsuits. But, some people aren't aware of this, and using the "Unlicense" makes it very clear to anyone that the original authors aren't claiming ownership of derivative works. For any "complex" code that we want to be free, we should use Apache. For simple code snippets and examples, the Unlicense will probably make casual/inexperienced developers feel more comfortable.
