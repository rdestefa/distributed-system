package main

import (
	"math"
)

const EPS float64 = 1e-8

var ZERO_VECTOR = Vector{X: 0, Y: 0}

type Vector struct {
	X float64
	Y float64
}

func (v Vector) add(other Vector) Vector {
	return Vector{
		X: v.X + other.X,
		Y: v.Y + other.Y,
	}
}

func (v Vector) sub(other Vector) Vector {
	return Vector{
		X: v.X - other.X,
		Y: v.Y - other.Y,
	}
}

func (v Vector) mul(scalar float64) Vector {
	return Vector{
		X: v.X * scalar,
		Y: v.Y * scalar,
	}
}

func (v Vector) crossProduct(other Vector) float64 {
	return v.X*other.Y - v.Y*other.X
}

func (v Vector) almostEqual(other Vector) bool {
	return math.Abs(v.X-other.X) < EPS && math.Abs(v.Y-other.Y) < EPS
}

type polygon struct {
	Points []Vector
	Edges  [][2]Vector
}

func newpolygon(points []Vector) *polygon {
	closed := points[0].almostEqual(points[len(points)-1])

	poly := &polygon{
		Points: make([]Vector, 0, len(points)),
		Edges:  make([][2]Vector, 0, len(points)),
	}

	for i, point := range points {
		if i == 0 || !point.almostEqual(points[i-1]) {
			poly.Points = append(poly.Points, point)
		}
	}

	if !closed {
		poly.Points = append(poly.Points, poly.Points[len(poly.Points)-1])
	}

	if len(poly.Points) <= 3 {
		return nil
	}

	for i := 1; i < len(poly.Points); i++ {
		poly.Edges = append(poly.Edges, [2]Vector{poly.Points[i-1], poly.Points[i]})
	}

	return poly
}

func (poly *polygon) inside(point Vector) bool {
	for _, edge := range poly.Edges {
		if edge[1].sub(edge[0]).crossProduct(point.sub(edge[0])) < EPS {
			return false
		}
	}

	return true
}

type Navmesh []*polygon

func newNavmesh(meshPoints [][][2]float64) *Navmesh {
	nav := make(Navmesh, 0, len(meshPoints))
	for _, polyPoints := range meshPoints {
		points := make([]Vector, 0, len(polyPoints))
		for _, point := range polyPoints {
			points = append(points, Vector{X: point[0], Y: point[1]})
		}
		poly := newpolygon(points)
		if poly != nil {
			nav = append(nav, poly)
		}
	}
	return &nav
}
