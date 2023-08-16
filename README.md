# REST Facades over GraphQL

## Converstion Rules

### Queries

 * Method is GET
 * Name to snake_case
 * If input containes id:ID!, add it to path like, /my_query/{id}
 * Non object parameters are pulled from query string
 * Object parameters are flattened to: input_variable_name.input_field
   (option, encode as JSON?)
 * Field selection defaults to all.  
   * Override with _fields=field1,field2,field3.inner1
   * Limit with _except=field4,field5
 * Any field in the tree that takes arguments other than id:ID! is considered 
   the end field of the route path.  This may cause certain parts of the graph to 
   be unreachable. 

 * Only one array field can exist within the path and it must be the last element.
   * /library/books[]
   * /library/:id/vault/secrets[]


/books/author/awards

[
    {
        //book
        author: {
            secrets: [
                {
                    *.*
                }
            ]
        }
    }
]

### Mutations

 * Method is POST
 * Body is JSON
 * Inputs are flatted into JSON object
   * myMutation(input: Blah{id, name, address, city, state, zip}) =>
     { "input": { "id":"...", "name":"...", "address":"...", ...}}
   * Optional flag to remove single entry root of 1-object-input mutations
 * Field selection set of return type defaults to all.

### Operations nested in types

```
# Psuedo SDL
type Mutation {
    MyType { MyOtherType { My3rdType { doThisThing(...) }}}
}
```
 * Convert to path hierarchy: /my_type/my_other_type/my_3rd_type/do_this_thing


### Todo

 [] Create OpenAPI endpoint