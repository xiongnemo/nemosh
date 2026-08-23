% a Prolog line comment
:- module(demo, [ancestor/2]).

/* a block comment
   spanning lines */

parent(tom, bob).
parent(bob, ann).

ancestor(X, Y) :- parent(X, Y).
ancestor(X, Y) :- parent(X, Z), ancestor(Z, Y).

greet :- format("hello ~w~n", ['world']), X = 0'a, Y = 0'\n, write(X-Y).
