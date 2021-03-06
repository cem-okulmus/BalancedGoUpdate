package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"time"

	jsoniter "github.com/json-iterator/go"

	. "github.com/cem-okulmus/BalancedGoUpdate/algorithms"
	. "github.com/cem-okulmus/BalancedGoUpdate/lib"
)

//hook for the json-iterator library
var json = jsoniter.ConfigCompatibleWithStandardLibrary

func logActive(b bool) {
	if b {
		log.SetOutput(os.Stderr)

		log.SetFlags(0)
	} else {

		log.SetOutput(ioutil.Discard)
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}

}

//Version indicates the version exported from the Git repository
var Version string

//Date indicates the build date exported from the Git repository
var Date string

//Build indicates the exact build when current version was compiled
var Build string

type labelTime struct {
	time  float64
	label string
}

func (l labelTime) String() string {
	return fmt.Sprintf("%s : %.5f ms", l.label, l.time)
}

func outputStanza(algorithm string, decomp Decomp, times []labelTime, graph Graph, gml string, json string, K int, skipCheck bool) {
	decomp.RestoreSubedges()

	fmt.Println("Used algorithm: " + algorithm + " @" + Version)
	fmt.Println("Result ( ran with K =", K, ")\n", decomp)

	// Print the times
	var sumTotal float64

	for _, time := range times {
		sumTotal = sumTotal + time.time
	}
	fmt.Printf("Time: %.5f ms\n", sumTotal)

	fmt.Println("Time Composition: ")
	for _, time := range times {
		fmt.Println(time)
	}

	fmt.Println("\nWidth: ", decomp.CheckWidth())
	var correct bool
	if !skipCheck {
		correct = decomp.Correct(graph)
	} else {
		correct = true
	}

	fmt.Println("Correct: ", correct)
	if correct && len(gml) > 0 {
		f, err := os.Create(gml)
		check(err)

		defer f.Close()
		f.WriteString(decomp.ToGML())
		f.Sync()
	}
	if correct && len(json) > 0 {
		f, err := os.Create(json)
		check(err)

		defer f.Close()
		f.Write(WriteDecomp(decomp))
		f.Sync()
	}
}

func main() {

	flagSet := flag.NewFlagSet("", flag.ContinueOnError)
	flagSet.SetOutput(ioutil.Discard)

	//Command-Line Argument Parsing
	cpuprofile := flagSet.String("cpuprofile", "", "write cpu profile to file")
	logging := flagSet.Bool("log", false, "turn on extensive logs")
	computeSubedges := flagSet.Bool("sub", false, "turn off subedge computation for global option")
	width := flagSet.Int("width", 0, "a positive, non-zero integer indicating the width of the GHD to search for")
	graphPath := flagSet.String("graph", "", "input (for format see hyperbench.dbai.tuwien.ac.at/downloads/manual.pdf)")
	// localBal := flagSet.Bool("local", false, "Use local BalSep algorithm")
	globalBal := flagSet.Bool("global", false, "Use global BalSep algorithm")
	balanceFactorFlag := flagSet.Int("balfactor", 2, "Changes the factor that balanced separator check uses, default 2")
	useHeuristic := flagSet.Int("heuristic", 0, "turn on to activate edge ordering\n\t1 ... Vertex Degree Ordering\n\t2 ... Max. Separator Ordering\n\t3 ... MCSO\n\t4 ... Edge Degree Ordering")
	gy?? := flagSet.Bool("g", false, "perform a GY?? reduct")
	typeC := flagSet.Bool("t", false, "perform a Type Collapse")
	hingeFlag := flagSet.Bool("h", false, "use hingeTree Optimization")
	numCPUs := flagSet.Int("cpu", -1, "Set number of CPUs to use")
	bench := flagSet.Bool("bench", false, "Benchmark mode, reduces unneeded output (incompatible with -log flag)")
	detKTest := flagSet.Bool("det", false, "Use DetKDecomp algorithm")
	localBIP := flagSet.Bool("localbip", false, "Used in combination with \"det\": turns on local subedge handling")
	// balDetTest := flagSet.Int("balDet", 0, "Use the Hybrid BalSep-DetK algorithm. Number indicates depth, must be ??? 1")
	// seqBalDetTest := flagSet.Int("seqBalDet", 0, "Use sequential Hybrid BalSep - DetK algorithm.")
	gml := flagSet.String("gml", "", "Output the produced decomposition into the specified gml file ")
	jsonFlag := flagSet.String("json", "", "Output the produced decomposition into the specified json file ")
	pace := flagSet.Bool("pace", false, "Use PACE 2019 format for graphs (see pacechallenge.org/2019/htd/htd_format/)")
	exact := flagSet.Bool("exact", false, "Compute exact width (width flag ignored)")
	approx := flagSet.Int("approx", 0, "Compute approximated width and set a timeout in seconds (width flag ignored)")
	decomp := flagSet.String("decomp", "", "A GHD to be used as input, with marked nodes for the ??-mutable subtree.")
	cache := flagSet.String("cache", "", "A binary representation of the internal cache, to be used during updating.")
	exportCache := flagSet.Bool("exportCache", false, "Export the internal Cache after algoritm has run.")
	nice := flagSet.Bool("nice", false, "Expects nice hypergraphs as input, which already specificy their resp. width.")
	ensemble := flagSet.Bool("ensemble", false, "Run ensemble of detk and localbip for update. Only used for that.")
	// meta := flagSet.Int("meta", 0, "meta parameter for LogKHybrid")

	parseError := flagSet.Parse(os.Args[1:])
	if parseError != nil {
		fmt.Print("Parse Error:\n", parseError.Error(), "\n\n")
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		defer pprof.StopCPUProfile()
	}

	if *bench { // no logging output when running benchmarks
		*logging = false
	}
	logActive(*logging)

	BalancedFactor := *balanceFactorFlag

	runtime.GOMAXPROCS(*numCPUs)

	// Outpt usage message if graph and width not specified
	if parseError != nil || *graphPath == "" || (*width <= 0 && !*exact && !*nice && *approx == 0) {
		out := fmt.Sprint("Usage of BalancedGo (", Version, ", https://github.com/cem-okulmus/BalancedGoUpdate/commit/",
			Build, ", ", Date, ")")
		fmt.Fprintln(os.Stderr, out)
		flagSet.VisitAll(func(f *flag.Flag) {
			if f.Name != "width" && f.Name != "graph" && f.Name != "exact" {
				return
			}
			s := fmt.Sprintf("%T", f.Value) // used to get type of flag
			if s[6:len(s)-5] != "bool" {
				fmt.Printf("  -%-10s \t<%s>\n", f.Name, s[6:len(s)-5])
			} else {
				fmt.Printf("  -%-10s \n", f.Name)
			}
			fmt.Println("\t" + f.Usage)
		})

		fmt.Println("\nOptional Arguments: ")
		flagSet.VisitAll(func(f *flag.Flag) {
			if f.Name == "width" || f.Name == "graph" || f.Name == "exact" {
				return
			}
			s := fmt.Sprintf("%T", f.Value) // used to get type of flag
			if s[6:len(s)-5] != "bool" {
				fmt.Printf("  -%-10s \t<%s>\n", f.Name, s[6:len(s)-5])
			} else {
				fmt.Printf("  -%-10s \n", f.Name)
			}
			fmt.Println("\t" + f.Usage)
		})

		return
	}

	dat, err := ioutil.ReadFile(*graphPath)
	check(err)

	var parsedGraph Graph
	var parseGraph ParseGraph

	if !*pace && !*nice {
		parsedGraph, parseGraph = GetGraph(string(dat))
	} else if *nice {
		parsedGraph, parseGraph, *width = GetNiceGraph(string(dat))
	} else {
		parsedGraph = GetGraphPACE(string(dat))
	}

	originalGraph := parsedGraph
	log.Println("BIP: ", parsedGraph.GetBIP())
	var reducedGraph Graph

	var times []labelTime

	var solverUpdate UpdateAlgorithm
	var solverEnsemble Algorithm
	var parsedDecomp Decomp
	var parsedCache Cache

	// Check if shortcut present, before applying heuristics
	if *decomp != "" {

		// Determine solver

		if *detKTest {
			det := &DetKDecomp{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor, SubEdge: *localBIP}

			if *ensemble {
				solverEnsemble = det
			}

			solverUpdate = det
		}

		// if *balDetTest > 0 {
		// 	balDet := &BalDetKDecomp{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor, Depth: *balDetTest - 1}
		// 	solverUpdate = balDet
		// }

		// read and parse decomposition

		dis, err2 := ioutil.ReadFile(*decomp)
		check(err2)

		startPars := time.Now()

		parsedDecomp = GetDecomp(dis, parsedGraph, parseGraph.Encoding)
		// fmt.Println("parsed Decomp", parsedDecomp)

		msecPars := time.Now().Sub(startPars).Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msecPars, label: "Parsing"})

		startCheck := time.Now()

		// Check if this decomp is already correct
		checkCorrectness := parsedDecomp.Correct(originalGraph)

		msecCheck := time.Now().Sub(startCheck).Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msecCheck, label: "Correctness Check"})

		if checkCorrectness {
			fmt.Println(" Parsed Decomposition already correct, skipping update computation.")

			outputStanza(solverUpdate.Name(), parsedDecomp, times, parsedGraph, *gml, *jsonFlag, *width, true)

			return
		}

		// get the cache

		if *cache != "" {
			dat, err3 := ioutil.ReadFile(*cache)
			check(err3)

			parsedCache = GetCache(dat)
		} else {
			parsedCache.Init()
		}

	}

	// Sorting Edges to find separators faster
	if *useHeuristic > 0 {
		var heuristicMessage string

		start := time.Now()
		switch *useHeuristic {
		case 1:
			parsedGraph.Edges = GetDegreeOrder(parsedGraph.Edges)
			heuristicMessage = "Using degree ordering as a heuristic"
			break
		case 2:
			parsedGraph.Edges = GetMaxSepOrder(parsedGraph.Edges)
			heuristicMessage = "Using max seperator ordering as a heuristic"
			break
		case 3:
			parsedGraph.Edges = GetMSCOrder(parsedGraph.Edges)
			heuristicMessage = "Using MSC ordering as a heuristic"
			break
		case 4:
			parsedGraph.Edges = GetEdgeDegreeOrder(parsedGraph.Edges)
			heuristicMessage = "Using edge degree ordering as a heuristic"
			break
		}
		d := time.Now().Sub(start)
		msec := d.Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msec, label: "Heuristic"})

		if !*bench {
			fmt.Println(heuristicMessage)
			fmt.Printf("Time for heuristic: %.5f ms\n", msec)
			fmt.Printf("Ordering: %v\n", parsedGraph.String())
		}
	}
	var removalMap map[int][]int
	// Performing Type Collapse
	if *typeC {
		count := 0
		reducedGraph, removalMap, count = parsedGraph.TypeCollapse()
		parsedGraph = reducedGraph
		if !*bench { // be silent when benchmarking
			fmt.Println("\n\n", *graphPath)
			fmt.Println("Graph after Type Collapse:")
			for _, e := range reducedGraph.Edges.Slice() {
				fmt.Printf("%v %v\n", e, Edge{Vertices: e.Vertices})
			}
			fmt.Print("Removed ", count, " vertex/vertices\n\n")
		}
	}

	var ops []GY??Reduct
	// Performing GY?? reduction
	if *gy?? {

		if *typeC {
			reducedGraph, ops = reducedGraph.GY??Reduct()
		} else {
			reducedGraph, ops = parsedGraph.GY??Reduct()
		}

		parsedGraph = reducedGraph
		if !*bench { // be silent when benchmarking
			fmt.Println("Graph after GY??:")
			fmt.Println(reducedGraph)
			fmt.Println("Reductions:")
			fmt.Print(ops, "\n\n")
		}

	}

	// Add all subdedges to graph
	if *globalBal && !*computeSubedges {
		parsedGraph = parsedGraph.ComputeSubEdges(*width)

		fmt.Println("Graph with subedges \n", parsedGraph)
	}

	var hinget Hingetree
	var msecHinge float64

	if *hingeFlag {
		startHinge := time.Now()

		hinget = GetHingeTree(parsedGraph)

		dHinge := time.Now().Sub(startHinge)
		msecHinge = dHinge.Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msecHinge, label: "Hingetree"})

		if !*bench {
			fmt.Println("Produced Hingetree: ")
			fmt.Println(hinget)
		}
	}

	if *decomp != "" {

		startSc := time.Now()

		scenes := parsedDecomp.SceneCreation(parsedGraph)

		dSc := time.Now().Sub(startSc)
		msec_sc := dSc.Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msec_sc, label: "Scene Creation"})

		fmt.Println("Extracted scenes: ", scenes.Len())

		if solverUpdate != nil {
			var decomp Decomp
			start := time.Now()

			if *ensemble {

				wait := make(chan Decomp)
				go func() {
					wait <- solverUpdate.FindDecompUpdate(parsedGraph, scenes, parsedCache)
				}()

				go func() {
					wait <- solverEnsemble.FindDecomp()
				}()

				decomp = <-wait
			} else {
				decomp = solverUpdate.FindDecompUpdate(parsedGraph, scenes, parsedCache)
			}

			d := time.Now().Sub(start)
			msec := d.Seconds() * float64(time.Second/time.Millisecond)
			times = append(times, labelTime{time: msec, label: "Decomposition"})

			if !reflect.DeepEqual(decomp, Decomp{}) {
				var result bool
				decomp.Root, result = decomp.Root.RestoreGY??(ops)
				if !result {
					fmt.Println("Partial decomp:", decomp.Root)

					log.Panicln("GY?? reduction failed")
				}
				decomp.Root, result = decomp.Root.RestoreTypes(removalMap)
				if !result {
					fmt.Println("Partial decomp:", decomp.Root)

					log.Panicln("Type Collapse reduction failed")
				}
			}

			outputStanza(solverUpdate.Name(), decomp, times, parsedGraph, *gml, *jsonFlag, *width, false)

			if len(*gml) > 0 { // export cache if gml is set too
				f, err := os.Create(*gml + ".cache")
				check(err)

				defer f.Close()
				out, err := json.Marshal(solverUpdate.GetCache())
				check(err)

				f.Write(out)
				f.Sync()
			}

			return
		}

		fmt.Println("No supported update Algorithm chosen.")
		return

	}

	var solver Algorithm

	// Check for multiple flags
	chosen := 0

	// if *balDetTest > 0 {
	// 	balDet := &BalDetKDecomp{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor, Depth: *balDetTest - 1}
	// 	solver = balDet
	// 	chosen++
	// }

	// if *seqBalDetTest > 0 {
	// 	seqBalDet := &SeqBalDetKDecomp{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor, Depth: *seqBalDetTest - 1}
	// 	solver = seqBalDet
	// 	chosen++
	// }

	if *detKTest {
		det := &DetKDecomp{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor, SubEdge: *localBIP}
		solver = det
		chosen++
	}

	// if *globalBal {
	// 	global := &BalSepGlobal{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor}
	// 	solver = global
	// 	chosen++
	// }

	// if *localBal {
	// 	local := &BalSepLocal{K: *width, Graph: parsedGraph, BalFactor: BalancedFactor}
	// 	solver = local
	// 	chosen++
	// }

	if chosen > 1 {
		fmt.Println("Only one algorithm may be chosen at a time. Make up your mind.")
		return
	}

	if solver != nil {
		var decomp Decomp
		start := time.Now()

		if *exact {
			k := 1

			solved := false
			for !solved {

				solver.SetWidth(k)

				if *hingeFlag {
					decomp = hinget.DecompHinge(solver, parsedGraph)
				} else {
					decomp = solver.FindDecomp()
				}

				solved = decomp.Correct(parsedGraph)
				k++
			}
			*width = k - 1 // for correct output
		} else if *approx > 0 {
			ch := make(chan int, 1)
			go func() {
				m := parsedGraph.Edges.Len()
				k := int(math.Ceil(float64(m) / 2))
				decomp = solver.FindDecomp()
				k = decomp.CheckWidth()
				solved := false

				var newDecomp Decomp
				for !solved {
					newK := k - 1

					solver.SetWidth(newK)

					if *hingeFlag {
						newDecomp = hinget.DecompHinge(solver, parsedGraph)
					} else {
						newDecomp = solver.FindDecomp()
					}
					if newDecomp.Correct(parsedGraph) {
						k = newDecomp.CheckWidth()
						decomp = newDecomp
					} else {
						solved = true
					}
				}
				ch <- k
			}()

			select {
			case res := <-ch:
				*width = res
			case <-time.After(time.Duration(*approx) * time.Second):
				*width = decomp.CheckWidth()
			}
		} else {
			if *hingeFlag {
				decomp = hinget.DecompHinge(solver, parsedGraph)
			} else {
				decomp = solver.FindDecomp()
			}
		}

		d := time.Now().Sub(start)
		msec := d.Seconds() * float64(time.Second/time.Millisecond)
		times = append(times, labelTime{time: msec, label: "Decomposition"})

		if !reflect.DeepEqual(decomp, Decomp{}) || (len(ops) > 0 && parsedGraph.Edges.Len() == 0) {
			var result bool
			decomp.Root, result = decomp.Root.RestoreGY??(ops)
			if !result {
				fmt.Println("Partial decomp:", decomp.Root)

				log.Panicln("GY?? reduction failed")
			}
			decomp.Root, result = decomp.Root.RestoreTypes(removalMap)
			if !result {
				fmt.Println("Partial decomp:", decomp.Root)

				log.Panicln("Type Collapse reduction failed")
			}
		}

		if !reflect.DeepEqual(decomp, Decomp{}) {
			decomp.Graph = originalGraph
		}
		outputStanza(solver.Name(), decomp, times, originalGraph, *gml, *jsonFlag, *width, false)

		if *exportCache { // export cache

			if *gml == "" {
				fmt.Println("Cannot export cache: You need to specify GML output location as well.")
				return
			}
			solverAsUpdate, ok := solver.(UpdateAlgorithm)

			if !ok {
				fmt.Println("Cannot export cache: Chosen algorithm does not support cache export.")
				return
			}

			f, err := os.Create(*gml + ".cache")
			check(err)
			defer f.Close()
			cache := solverAsUpdate.GetCache()

			out, err := json.Marshal(cache)
			check(err)

			f.Write(out)
			f.Sync()
		}
		return
	}

	fmt.Println("No algorithm or procedure selected.")

}
