# D2 (Declarative Diagramming) - Comprehensive Cheat Sheet

**Version:** D2 v0.7+ | **Last Updated:** October 2025

---

## Resources

- **Documentation:** [d2lang.com/tour](https://d2lang.com/tour)
- **GitHub:** [github.com/terrastruct/d2](https://github.com/terrastruct/d2)
- **Icon Library:** [icons.terrastruct.com](https://icons.terrastruct.com)

---

## Quick Start

```d2
# Basic connection
hello -> world

# With labels
hello: Hi -> world: Bye
```

---

## Shapes

### Basic Declaration
```d2
# Simple shape
server

# With label
db: Database

# Multiple shapes on one line
frontend; backend; database
```

### Shape Types
```d2
shape: rectangle      # default
shape: square
shape: circle
shape: oval
shape: diamond
shape: parallelogram
shape: hexagon
shape: octagon
shape: triangle
shape: cylinder
shape: queue
shape: page
shape: package
shape: cloud
shape: callout
shape: step
shape: text          # standalone text
shape: code          # code block
shape: class         # UML class
shape: sql_table     # database table
shape: sequence_diagram  # sequence diagram container
```

### Dimensions
```d2
server: {
  width: 100
  height: 200
}
```

---

## Connections

### Connection Types
```d2
a -- b    # undirected
a -> b    # directed
a <- b    # reverse directed
a <-> b   # bidirectional
```

### Connection Labels
```d2
a -> b: connection label

# Chained connections
a -> b -> c -> d
```

### Repeated Connections
```d2
# Creates multiple connections, not override
a -> b
a -> b
```

### Arrowheads
```d2
x -> y: {
  source-arrowhead: triangle    # default, can be unfilled
  target-arrowhead: arrow       # pointier triangle
}

# Available arrowheads:
# - triangle (default)
# - arrow
# - diamond (unfilled or filled)
# - circle (unfilled or filled)
# - box (unfilled or filled)
# - cf-one, cf-one-required (crow's foot)
# - cf-many, cf-many-required

# Arrowhead labels
x -> y: {
  source-arrowhead: {
    shape: diamond
    label: 1
  }
  target-arrowhead: {
    shape: diamond
    label: n
    style.filled: true
  }
}
```

### Referencing Connections
```d2
# First connection: (a -> b)[0]
# Second connection: (a -> b)[1]
(a -> b)[0].style.stroke: red
```

---

## Containers

### Method 1: Dot Notation
```d2
aws.server
aws.database
```

### Method 2: Map Syntax
```d2
aws: {
  server
  database
  region: us-east-1
}
```

### Nested Containers
```d2
cloud: {
  vpc: {
    subnet: {
      instance
    }
  }
}

# Connect across containers
cloud.vpc.subnet.instance -> database
```

---

## Styling

### Complete Style Properties
```d2
shape: {
  style: {
    opacity: 0.5              # 0-1
    stroke: "#FF5733"         # color/hex/gradient
    fill: blue                # color/hex/gradient
    fill-pattern: dots        # dots/lines/grain/none
    stroke-width: 3           # 1-15
    stroke-dash: 5            # 0-10
    border-radius: 10         # 0-20
    shadow: true              # true/false
    3d: true                  # true/false (rect/square only)
    multiple: true            # stacked effect
    double-border: true       # double outline
    font: mono                # mono (more coming)
    font-size: 24             # 8-100
    font-color: red           # color/hex
    animated: true            # true/false
    bold: true                # true/false
    italic: true              # true/false
    underline: true           # true/false
    text-transform: uppercase # uppercase/lowercase/title/none
  }
}
```

### Gradient Examples
```d2
server: {
  style.fill: "#8BC34A;#0D47A1"  # linear gradient
}
```

### Root-Level Styles
```d2
style: {
  fill: "#f0f0f0"         # diagram background
  fill-pattern: dots      # background pattern
  stroke: black           # frame color
  stroke-width: 2         # frame width
  stroke-dash: 3          # dashed frame
  double-border: true     # double frame
}
```

---

## Icons & Images

```d2
# From URL
server: {
  icon: https://icons.terrastruct.com/aws/Compute/EC2.svg
}

# Local file
database: {
  icon: ./images/db-icon.png
}

# Icon positioning
server: {
  icon: https://example.com/icon.png
  icon.near: top-left
}
```

**Icon Library:** [icons.terrastruct.com](https://icons.terrastruct.com)

---

## Text, Markdown & Code

### Markdown
```d2
explanation: |md
  # Header
  - List item
  - **Bold** and *italic*
  
  Code: `inline code`
|
```

### Code Blocks
```d2
api: |go
  func main() {
    fmt.Println("Hello D2")
  }
|

# Supported languages: go, py, js, rb, ts, java, c, cpp, rust, etc.
# Full list: github.com/alecthomas/chroma
```

### LaTeX
```d2
formula: |latex
  E = mc^2
  
  \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}
|

# Font sizing in LaTeX
equation: |tex
  \huge{x = 5}
  \large{y = 10}
|
```

### Block Strings (Multi-line)
```d2
# Standard
text: |
  Line 1
  Line 2
|

# Custom delimiters (for languages using |)
code: ||
  if (x || y) { }
||

# Triple pipe
script: |||
  // even more complex
|||

# Any special character after first pipe
content: |###
  custom delimiter
|###
```

### Plain Text (Non-Markdown)
```d2
note: {
  shape: text
}
note: This is plain text, not markdown
```

---

## Special Shapes

### SQL Tables
```d2
users: {
  shape: sql_table
  
  # Columns with constraints
  id: int {constraint: primary_key}
  email: varchar {constraint: unique}
  created_at: timestamp
  department_id: int {constraint: foreign_key}
}

# Relationships
users.department_id -> departments.id
```

### UML Classes
```d2
DatabaseManager: {
  shape: class
  
  # Public (+), Private (-), Protected (#)
  +connection: Connection
  -poolSize: int
  #timeout: int
  
  +connect(): void
  +disconnect(): void
  -validateConnection(): bool
  #reconnect(): void
}
```

---

## Sequence Diagrams

```d2
sequence: {
  shape: sequence_diagram
  
  # Define actors (optional, for ordering)
  alice
  bob
  
  # Messages
  alice -> bob: Hello
  bob -> alice: Hi there!
  
  # Self-messages
  alice -> alice: thinking...
  
  # Spans (lifespans/activation boxes)
  bob."critical section": {
    alice -> bob."critical section": request
    bob."critical section" -> alice: response
  }
  
  # Notes
  alice.note: {
    shape: text
  }
  alice.note: Remember to respond
  
  # Groups (fragments)
  group1: {
    alice -> bob: message in group
  }
  group1.label: Optional Operation
}
```

---

## Interactive Features

### Tooltips
```d2
server: {
  tooltip: "This is a web server running nginx"
}
```

### Links
```d2
docs: {
  link: https://d2lang.com
}

# With URL fragments (must quote or escape #)
page: {
  link: "https://example.com/page#section"
}
```

---

## Animations

```d2
# Animated shape
loading: {
  style.animated: true
}

# Animated connection
a -> b: {
  style.animated: true
}
```

---

## Comments

```d2
# Single line comment

# Multi-line description
# continues here
# and here

server # inline comment (not recommended)
```

---

## Positioning & Layout

### Near Keyword
```d2
x; y; z

x.label.near: outside-top-left
y.icon.near: top-center
z.label.near: inside-bottom-right

# Positions: 
# top-left, top-center, top-right
# center-left, center-right
# bottom-left, bottom-center, bottom-right
# inside-* or outside-* variants
```

### Direction
```d2
direction: right  # up/down/left/right (default: down)

container: {
  direction: down  # can set per container
  a -> b -> c
}
```

---

## Themes

```d2
# Available themes (100+ built-in):
# 0: Neutral default
# 1: Neutral grey
# 2: Flagship terrastruct
# 3: Cool classics
# 4: Mixed berry blue
# 5-7: Various professional themes
# 8: Earth tones
# 100-200+: Community themes
# 300-400+: Sketch themes
```

---

## Advanced Features

### Globs (Pattern Matching)
```d2
# Style all children
*.style.fill: blue

# Nested glob
aws.*.style.stroke: red

# Change defaults
**.style.border-radius: 8
```

### Imports
```d2
# Import another D2 file
...@common.d2

# Import into specific scope
aws: {
  ...@aws-resources.d2
}
```

### Variables (Vars)
```d2
vars: {
  primary-color: "#FF5733"
  server-width: 150
}

server: {
  style.fill: ${primary-color}
  width: ${server-width}
}
```

### Layers & Scenarios
```d2
layers: {
  base: {
    a -> b
  }
  detailed: {
    a -> b -> c -> d
  }
}

scenarios: {
  normal: {
    server.style.fill: green
  }
  error: {
    server.style.fill: red
  }
}
```

### Reserved Keywords (Quote if Using as ID)
```d2
"shape": This is okay
"style": Works when quoted
"label": Can be ID when quoted
"width": Quote these reserved words
"height"
"icon"
"tooltip"
"link"
"near"
"direction"
```

---

## Tips & Tricks

### 1. Keep Keys Case-Insensitive
```d2
# These reference the same shape
PostgreSQL -> MySQL
postgresql -> mysql
```

### 2. Use Semicolons for Compact Definitions
```d2
frontend; backend; database; cache
```

### 3. Connection vs Shape Reference
```d2
# Creates shape first, then connects
server
database
server -> database

# Or in one line
server -> database  # creates both if they don't exist
```

### 4. Transparent Fill
```d2
box: {
  style.fill: transparent
}
```

### 5. High Border-Radius for Pills
```d2
button: {
  style.border-radius: 20
}
```

---

## Example: Complete Diagram

```d2
direction: right

# Root style
style: {
  fill: "#f5f5f5"
  stroke: "#333"
}

title: Web Application Architecture {
  shape: text
  near: top-center
  style.font-size: 32
  style.bold: true
}

# Frontend
frontend: Frontend {
  icon: https://icons.terrastruct.com/dev/react.svg
  style.fill: "#61DAFB"
  style.multiple: true
  
  tooltip: "React SPA"
}

# API Layer
api: API Gateway {
  shape: hexagon
  icon: https://icons.terrastruct.com/aws/Networking%20&%20Content%20Delivery/API-Gateway.svg
  style.fill: "#FF9900"
  style.3d: true
}

# Backend Services
backend: Backend Services {
  direction: down
  style.fill: "#00758F"
  
  auth: Auth Service {
    icon: https://icons.terrastruct.com/aws/Security%2C%20Identity%2C%20&%20Compliance/AWS-Identity-and-Access-Management_IAM.svg
  }
  
  users: User Service {
    icon: https://icons.terrastruct.com/aws/Compute/EC2.svg
  }
  
  payments: Payment Service {
    icon: https://icons.terrastruct.com/aws/Blockchain/Amazon-Managed-Blockchain.svg
  }
}

# Database
db: PostgreSQL {
  shape: cylinder
  icon: https://icons.terrastruct.com/dev/postgresql.svg
  style.fill: "#336791"
}

# Cache
cache: Redis Cache {
  shape: package
  icon: https://icons.terrastruct.com/dev/redis.svg
  style.fill: "#DC382D"
}

# Connections
frontend -> api: HTTPS {
  style.animated: true
}

api -> backend.auth: Authenticate
api -> backend.users: User CRUD
api -> backend.payments: Process Payment

backend.auth -> db
backend.users -> db
backend.payments -> db

backend.users -> cache: Cache Layer {
  style.stroke-dash: 3
}

# Notes
note: |md
  ## System Notes
  - All services use JWT
  - Redis for session storage
  - PostgreSQL for persistence
| {
  near: bottom-right
}
```
