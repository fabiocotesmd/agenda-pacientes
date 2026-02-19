# Introducción a Go (Golang)

## ¿Qué es Go?

Go, también conocido como Golang, es un lenguaje de programación de código abierto desarrollado por Google en 2007 y lanzado públicamente en 2009. Fue diseñado por Robert Griesemer, Rob Pike y Ken Thompson con el objetivo de crear un lenguaje simple, eficiente y adecuado para el desarrollo de software moderno.

## Características Principales

### 1. **Simplicidad**
- Sintaxis limpia y minimalista
- Fácil de aprender, especialmente si conoces C, Java o JavaScript
- Menos palabras clave que otros lenguajes (solo 25)

### 2. **Rendimiento**
- Compilado a código nativo
- Velocidad comparable a C/C++
- Gestión eficiente de memoria con garbage collector

### 3. **Concurrencia**
- Soporte nativo para programación concurrente mediante goroutines
- Canales (channels) para comunicación segura entre goroutines
- Modelo de concurrencia simple y poderoso

### 4. **Compilación Rápida**
- Tiempos de compilación extremadamente rápidos
- Binario único independiente (sin dependencias externas)

### 5. **Tipado Estático**
- Detección de errores en tiempo de compilación
- Inferencia de tipos para mayor comodidad

### 6. **Herramientas Integradas**
- Formateador de código (`gofmt`)
- Gestor de dependencias (`go mod`)
- Sistema de testing integrado
- Documentación automática (`godoc`)

## ¿Para Qué se Usa Go?

Go es especialmente popular en:

- **Desarrollo de servicios backend y APIs**
- **Microservicios y arquitecturas distribuidas**
- **Herramientas de línea de comandos (CLI)**
- **DevOps y automatización** (Docker, Kubernetes están escritos en Go)
- **Cloud computing y servicios web**
- **Sistemas de red y servidores**

## Estructura Básica de un Programa en Go

```go
package main

import "fmt"

func main() {
    fmt.Println("¡Hola, mundo!")
}
```

### Componentes Clave:
- `package main`: Define el paquete principal
- `import`: Importa bibliotecas necesarias
- `func main()`: Punto de entrada del programa

## Primeros Pasos

### Instalación
1. Descarga Go desde [golang.org/dl](https://golang.org/dl/)
2. Sigue las instrucciones de instalación para tu sistema operativo
3. Verifica la instalación: `go version`

### Tu Primer Programa
```bash
# Crear un nuevo módulo
go mod init ejemplo

# Crear archivo main.go con el código de arriba
# Ejecutar el programa
go run main.go
```

## Conceptos Fundamentales

### Variables
```go
var nombre string = "Go"
edad := 15  // declaración corta con inferencia de tipos
```

### Tipos de Datos Básicos
- `bool`: valores booleanos
- `string`: cadenas de texto
- `int`, `int8`, `int16`, `int32`, `int64`: enteros
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`: enteros sin signo
- `float32`, `float64`: números de punto flotante
- `complex64`, `complex128`: números complejos

### Estructuras de Control
```go
// Condicionales
if x > 0 {
    fmt.Println("Positivo")
} else {
    fmt.Println("No positivo")
}

// Bucles (solo existe 'for')
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

// Switch
switch dia {
case "lunes":
    fmt.Println("Inicio de semana")
default:
    fmt.Println("Otro día")
}
```

### Funciones
```go
func sumar(a int, b int) int {
    return a + b
}

// Múltiples valores de retorno
func dividir(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("división por cero")
    }
    return a / b, nil
}
```

## Ventajas de Aprender Go

✅ **Alta demanda laboral**: Muchas empresas buscan desarrolladores Go
✅ **Salarios competitivos**: Especialización bien remunerada
✅ **Comunidad activa**: Gran soporte y recursos disponibles
✅ **Versatilidad**: Útil para múltiples dominios de programación
✅ **Futuro prometedor**: Adopción creciente en la industria

---

# 📚 Fuentes Educativas Online

## Recursos Oficiales

### 1. **Go.dev - Documentación Oficial**
- **URL**: https://go.dev/
- **Contenido**: Documentación oficial, tutoriales y guías
- **Nivel**: Todos los niveles
- **Idioma**: Inglés

### 2. **A Tour of Go**
- **URL**: https://go.dev/tour/
- **Contenido**: Tutorial interactivo oficial en el navegador
- **Nivel**: Principiante
- **Idioma**: Inglés (con traducciones disponibles)
- **Destacado**: ⭐ Ideal para empezar, no requiere instalación

### 3. **Go by Example**
- **URL**: https://gobyexample.com/
- **Contenido**: Ejemplos prácticos de código con explicaciones
- **Nivel**: Principiante a Intermedio
- **Idioma**: Inglés
- **Destacado**: ⭐ Excelente referencia rápida

### 4. **Effective Go**
- **URL**: https://go.dev/doc/effective_go
- **Contenido**: Guía de mejores prácticas y estilo
- **Nivel**: Intermedio
- **Idioma**: Inglés

## Cursos en Video

### 5. **freeCodeCamp - Learn Go Programming (YouTube)**
- **URL**: https://www.youtube.com/watch?v=YS4e4q9oBaU
- **Duración**: ~7 horas
- **Nivel**: Principiante
- **Idioma**: Inglés
- **Costo**: Gratuito

### 6. **Coursera - Programming with Google Go Specialization**
- **URL**: https://www.coursera.org/specializations/google-golang
- **Contenido**: Especialización de 3 cursos de UC Irvine
- **Nivel**: Principiante a Intermedio
- **Idioma**: Inglés (con subtítulos)
- **Costo**: Auditoría gratuita, certificado de pago

### 7. **Udemy - Go: The Complete Developer's Guide**
- **URL**: https://www.udemy.com/course/go-the-complete-developers-guide/
- **Contenido**: Curso completo de Go
- **Nivel**: Principiante
- **Idioma**: Inglés
- **Costo**: De pago (frecuentes descuentos)

## Plataformas Interactivas

### 8. **Exercism - Go Track**
- **URL**: https://exercism.org/tracks/go
- **Contenido**: Ejercicios prácticos con mentoría
- **Nivel**: Principiante a Avanzado
- **Idioma**: Inglés
- **Costo**: Gratuito
- **Destacado**: ⭐ Feedback personalizado de mentores

### 9. **Codecademy - Learn Go**
- **URL**: https://www.codecademy.com/learn/learn-go
- **Contenido**: Curso interactivo en navegador
- **Nivel**: Principiante
- **Idioma**: Inglés
- **Costo**: Freemium (contenido básico gratis)

### 10. **LeetCode - Go Problems**
- **URL**: https://leetcode.com/
- **Contenido**: Problemas de algoritmos y estructuras de datos
- **Nivel**: Intermedio a Avanzado
- **Idioma**: Inglés
- **Costo**: Freemium

## Libros Online Gratuitos

### 11. **The Go Programming Language (Donovan & Kernighan)**
- **Referencia**: Libro considerado "la biblia" de Go
- **Nivel**: Intermedio a Avanzado
- **Nota**: Libro de pago, pero altamente recomendado

### 12. **Learning Go - Jon Bodner (O'Reilly)**
- **URL**: Disponible en O'Reilly Learning Platform
- **Nivel**: Principiante a Intermedio
- **Idioma**: Inglés

### 13. **Go Bootcamp**
- **URL**: http://www.golangbootcamp.com/
- **Contenido**: Libro gratuito online
- **Nivel**: Principiante
- **Idioma**: Inglés

## Recursos en Español

### 14. **Go en Español (Documentación traducida)**
- **URL**: https://go-es.dev/
- **Contenido**: Recursos y documentación en español
- **Nivel**: Todos los niveles
- **Idioma**: Español

### 15. **AprendeGo.dev**
- **URL**: https://aprendego.dev/
- **Contenido**: Tutoriales y recursos en español
- **Nivel**: Principiante
- **Idioma**: Español

### 16. **YouTube - Código Facilito (Go)**
- **URL**: Buscar "Golang" en YouTube (varios canales)
- **Contenido**: Tutoriales en video
- **Nivel**: Principiante a Intermedio
- **Idioma**: Español

## Blogs y Comunidades

### 17. **The Go Blog**
- **URL**: https://go.dev/blog/
- **Contenido**: Artículos oficiales del equipo de Go
- **Nivel**: Todos los niveles
- **Idioma**: Inglés

### 18. **r/golang (Reddit)**
- **URL**: https://reddit.com/r/golang
- **Contenido**: Comunidad activa, noticias y discusiones
- **Nivel**: Todos los niveles
- **Idioma**: Inglés

### 19. **Gophers Slack**
- **URL**: https://gophers.slack.com/
- **Contenido**: Comunidad en Slack
- **Nivel**: Todos los niveles
- **Idioma**: Inglés
- **Registro**: https://invite.slack.golangbridge.org/

### 20. **Stack Overflow - Go Tag**
- **URL**: https://stackoverflow.com/questions/tagged/go
- **Contenido**: Preguntas y respuestas
- **Nivel**: Todos los niveles
- **Idioma**: Principalmente inglés

## Canales de YouTube

### 21. **Learn To Code - Golang Training**
- **URL**: https://www.youtube.com/@ToddMcLeod
- **Contenido**: Cursos completos de Go
- **Nivel**: Principiante a Avanzado
- **Idioma**: Inglés

### 22. **TechWorld with Nana - Golang Tutorial**
- **URL**: https://www.youtube.com/c/TechWorldwithNana
- **Contenido**: Tutoriales prácticos
- **Nivel**: Principiante a Intermedio
- **Idioma**: Inglés

## Práctica y Proyectos

### 23. **Awesome Go**
- **URL**: https://awesome-go.com/
- **Contenido**: Lista curada de frameworks, bibliotecas y recursos
- **Nivel**: Todos los niveles
- **Idioma**: Inglés

### 24. **GitHub - Golang Projects**
- **URL**: https://github.com/topics/golang
- **Contenido**: Proyectos open source en Go
- **Nivel**: Intermedio a Avanzado
- **Idioma**: Código (mayormente inglés en documentación)

## Ruta de Aprendizaje Sugerida

1. **Semana 1-2**: A Tour of Go + Go by Example
2. **Semana 3-4**: Curso en video (freeCodeCamp o similar)
3. **Semana 5-8**: Exercism + proyectos pequeños propios
4. **Mes 3+**: Leer Effective Go + contribuir a proyectos open source

---

## Consejos para Aprender Go

💡 **Practica diariamente**: Incluso 30 minutos al día hacen la diferencia
💡 **Lee código de otros**: Explora proyectos en GitHub
💡 **Construye proyectos**: La mejor forma de aprender es haciendo
💡 **Únete a la comunidad**: Participa en foros y grupos
💡 **Sigue las convenciones**: Usa `gofmt` y sigue el estilo idiomático de Go

---

**Última actualización**: Febrero 2026
**Licencia**: Documento educativo de uso libre
