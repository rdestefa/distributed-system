package main

import "math"

const EPS float64 = 1e-8

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
		Points: make([]Vector, len(points)),
		Edges:  make([][2]Vector, 0, len(points)),
	}

	copy(poly.Points, points)

	if !closed {
		poly.Points = append(poly.Points, poly.Points[len(poly.Points)-1])
	}

	for i := 1; i <= len(poly.Points); i++ {
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
		points := make([]Vector, len(polyPoints))
		for _, point := range polyPoints {
			points = append(points, Vector{X: point[0], Y: point[1]})
		}
		nav = append(nav, newpolygon(points))
	}
	return &nav
}
