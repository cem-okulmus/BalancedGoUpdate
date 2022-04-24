# BalancedGoUpdate

This is a a modified version of BalancedGo (https://github.com/cem-okulmus/BalancedGo). This version enables efficient updating of decompositions when the underlying hypergraph has been modified. 

This version takes as input an hypergraph (with the understanding that it has been updated/modified in some form), a GHD of the original hypergraph in JSON format with the minimal mutable subtree explicitely marked and a width parameter (a non-zero positive integer).

Test instances can be found in the experimental data that is available here: https://zenodo.org/record/6481125. The hypergraphs that have been modified as part of our experiments originally stem from the HyperBench dataset, available here: http://hyperbench.dbai.tuwien.ac.at/. 

## Installation

Needs Go >= 1.12, look [here](https://golang.org/dl/) for Linux, MacOS or Windows versions.   
Simply run `make`, alternatively on platforms without the make tool, run `go build`
