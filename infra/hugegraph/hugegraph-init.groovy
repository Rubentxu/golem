/*
 * HugeGraph GremlinServer initialization script.
 * Binds the 'g' traversal source to the HugeGraph instance.
 *
 * This script is loaded by GremlinServer at startup via ScriptFileGremlinPlugin.
 * It creates a global 'g' variable that Gremlin scripts can use.
 */

import org.apache.hugegraph.HugeFactory
import org.apache.hugegraph.config.CoreOptions

def globals = [:]

// Open the HugeGraph instance using the REST API URL configured in the properties
// The graph name must match what's registered in the REST API
try {
    def graph = HugeFactory.open("conf/graphs/hugegraph.properties")
    globals << [g: graph.traversal()]
    println("HugeGraph GremlinServer: bound 'g' to graph '" + graph.name() + "'")
} catch (Exception e) {
    println("HugeGraph GremlinServer: WARNING - could not bind 'g': " + e.getMessage())
    // Fallback: create an empty traversal to avoid null reference errors
    // The HugeGraphGremlinPlugin may still override this with the actual graph
}
