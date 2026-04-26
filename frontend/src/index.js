import React from 'react';
import ReactDOM from 'react-dom';

import {
  isMobile,
} from 'react-device-detect';

import {
  MIN_SEARCH_LEN,
  API_HOST,
  DEBUG_LOG,
} from './api';

import '@fortawesome/fontawesome-free/css/all.min.css'; 
import 'bootstrap-css-only/css/bootstrap.min.css'; 
import 'mdbreact/dist/css/mdb.css';
import './index.css';

import {
  MDBInput, 
} from 'mdbreact';


class SeriesSearch extends React.Component {
  
  constructor(props) {
    super(props);
    this.state = {
      search_input: "",
      found_series: [],
      search_err: null,
      show_on_flow: false,
      on_the_flow_selected_idx: -1,
      language: "en",
      found_series_commited: [],
      search_err_commited: null,
      show_final: false,

      mouse_hover_blocked: !isMobile,
    };
    this.handleEscPress = this.handleEscPress.bind(this);
    this.goToNextDropdownItem = this.goToNextDropdownItem.bind(this);
    this.goToPrevDropdownItem = this.goToPrevDropdownItem.bind(this);
  }

  blockMouseHover = () => {
    if (DEBUG_LOG) {
      console.log("block mouse hover");
    }
    if (!isMobile) {
      this.setState({
        mouse_hover_blocked: true,
      });
    }
  }
  unblockMouseHover = () => {
    if (DEBUG_LOG) {
      console.log("unblock mouse hover");
    }
    this.setState({
      mouse_hover_blocked: false,
    });
  }

  doSearch = (search_str) => {
    fetch(API_HOST+`/search/${search_str}`)
      .then(resp => {
        console.log("search resp", resp)
        let json = resp.json();
        if (DEBUG_LOG) {
          console.log(json);
        }
        return json;
      })
      .then(resp_json => {
        if (DEBUG_LOG) {
          console.log(resp_json);
        }
        this.setState({
          found_series: resp_json.searchResults,
          language: resp_json.language,
          show_on_flow: (resp_json && resp_json.searchResults && resp_json.searchResults.length > 0),
          search_err: null,
        });
      });
  }

  handleSearchInputChange = (e) => {
    let search_str = e.target.value;
    search_str = search_str.trimLeft();
    if (DEBUG_LOG) {
      console.log("input changed to", search_str);
    }

    this.blockMouseHover();

    this.setState({
      search_input: search_str,
      show_on_flow: false,
      on_the_flow_selected_idx: -1,
    }, () => {
      if (search_str.length < MIN_SEARCH_LEN) {
        this.setState({
          found_series: [],
          language: this.state.language,
          search_err: null,
        });
        return;
      }
      this.doSearch(search_str);
    });
  }

  doFinalSearch = (e) => {
    if (e.key !== 'Enter') {
      return;
    }
    e.preventDefault();
    if (DEBUG_LOG) {
      console.log("doFinalSearch of ", this.state.search_input);
    }

    if (this.state.search_input === "") {
      return;
    }

    const doSearchAndSetFinal = (search_str) => {
      fetch(API_HOST+`/search/${search_str}`)
        .then(resp => {
          console.log("search resp", resp);
          let json = resp.json();
          if (DEBUG_LOG) {
            console.log(json);
          }
          return json;
        })
        .then(resp_json => {
          if (DEBUG_LOG) {
            console.log(resp_json);
          }
          this.setState({
            search_input: "",
            show_on_flow: false,
            found_series: [],
            search_err: null,
            show_final: true, 
            found_series_commited: resp_json.searchResults,
            language: resp_json.language,
            search_err_commited: null,
          });
        })
        .catch(() => {
          console.log("search fetch error");
          this.setState({
            show_on_flow: false,
            found_series: [],
            language: this.state.language,
            search_err: null,
            show_final: true, 
            found_series_commited: [],
            search_err_commited: "Service unavailable",
          });
        });
    }

    if (this.state.on_the_flow_selected_idx >= 0) {
      let new_found_series = this.state.found_series;
      new_found_series[this.state.on_the_flow_selected_idx].selected = false;
      new_found_series[this.state.on_the_flow_selected_idx].not_selectable = true;
      let selected_imdb_id = new_found_series[this.state.on_the_flow_selected_idx].id;
      let callback = () => {
        return this.doFinalSeriesSelect(selected_imdb_id);
      }
      this.setState({
        show_on_flow: false,
        // search_input: this.state.found_series[this.state.on_the_flow_selected_idx].title,
        search_input: "",
        found_series: [],
        search_err: null,
        on_the_flow_selected_idx: -1,
        show_final: true,
        language: this.state.language,
        found_series_commited: [new_found_series[this.state.on_the_flow_selected_idx]],
        search_err_commited: null,
      }, callback);
    } else {
      doSearchAndSetFinal(this.state.search_input);
    }
  }

  handleOnTheFlowSelection = (e) => {
    if (e.keyCode === 40) { // down
      if (DEBUG_LOG) {
        console.log("down arrow pressed");
      }
      this.goToNextDropdownItem();
    } else if (e.keyCode === 38) { // up
      if (DEBUG_LOG) {
        console.log("up arrow pressed");
      }
      this.goToPrevDropdownItem();
    }
  }

  hadleSeriesItemHover = (e) => {
    if (this.state.mouse_hover_blocked) {
      return;
    }
    let selected_idx = parseInt(e.target.dataset.idx);
    if (DEBUG_LOG) {
      console.log("mouse hover on", selected_idx);
    }
    let prev_selection_idx = this.state.on_the_flow_selected_idx;
    if (selected_idx !== prev_selection_idx) {
      let new_found_series = this.state.found_series;
      if (prev_selection_idx >= 0) {
        new_found_series[prev_selection_idx].selected = false;
      }
      new_found_series[selected_idx].selected = true;
      this.setState({
        on_the_flow_selected_idx: selected_idx,
        found_series: new_found_series,
        language: this.state.language,
      });
    }
  }

  handleDropdownMouseLeave = (e) => {
    if (DEBUG_LOG) {
      console.log("mouse leaved dropdown");
    }
    let prev_selection_idx = this.state.on_the_flow_selected_idx;
    if (prev_selection_idx < 0) {
      return;
    }
    let new_found_series = this.state.found_series;
    new_found_series[prev_selection_idx].selected = false;
    this.setState({
      on_the_flow_selected_idx: -1,
      found_series: new_found_series,
      language: this.state.language,
    });
  }

  goToNextDropdownItem = () => {
    if (this.state.found_series.length === 0) {
      return;
    }
    if (this.state.on_the_flow_selected_idx >= this.state.found_series.length-1) {
      return;
    }
    let prev_selection_idx = this.state.on_the_flow_selected_idx;
    let new_found_series = this.state.found_series;
    if (prev_selection_idx >= 0) {
      new_found_series[prev_selection_idx].selected = false;
    }
    new_found_series[prev_selection_idx+1].selected = true;
    this.setState({
      on_the_flow_selected_idx: prev_selection_idx+1,
      found_series: new_found_series,
      language: this.state.language,
    }, () => {DEBUG_LOG && console.log("selected_idx:", this.state.on_the_flow_selected_idx);});
  }

  goToPrevDropdownItem = () => {
    if (this.state.found_series.length === 0) {
      return;
    }
    if (this.state.on_the_flow_selected_idx < 0 ) {
      return;
    }
    let prev_selection_idx = this.state.on_the_flow_selected_idx;
    let new_found_series = this.state.found_series;
    new_found_series[prev_selection_idx].selected = false;
    if (prev_selection_idx > 0) {
      new_found_series[prev_selection_idx-1].selected = true;
    }
    this.setState({
      on_the_flow_selected_idx: prev_selection_idx-1,
      found_series: new_found_series,
      language: this.state.language,
    }, () => {DEBUG_LOG && console.log("selected_idx:", this.state.on_the_flow_selected_idx);});
  }

  doFinalSeriesSelect = (imdb_id) => {
    if (DEBUG_LOG) {
      console.log("doFinalSeriesSelect of ", imdb_id);
    }
    this.props.onSeriesSelect(imdb_id, this.state.language);
  }

  handleSeriesSelect = (e) => {
    if (DEBUG_LOG) {
      console.log("handleSeriesSelect", this.state.on_the_flow_selected_idx);
    }
    let selected_idx = parseInt(e.target.dataset.idx);
    let selected_series = this.state.found_series[selected_idx];
    selected_series.not_selectable = true;
    if (DEBUG_LOG) {
      console.log(`e.idx: ${e.target.dataset.idx} selected_idx: ${selected_idx}, selected_series: ${selected_series}`);
      console.log("selected series ", selected_series.title, " with imdb_id ", selected_series.id);
    }
    this.setState({
      search_input: "",
      show_final: true, 
      show_on_flow: false,
      language: this.state.language,
      found_series_commited: [selected_series],
      search_err_commited: null,
    }, this.props.onSeriesSelect(selected_series.id, this.state.language));
  }

  handleSeriesCommitedSelect = (e) => {
    let selected_idx = parseInt(e.target.dataset.idx);
    let selected_series = this.state.found_series_commited[selected_idx];
    selected_series.not_selectable = true;
    if (DEBUG_LOG) {
      console.log(this.state.language)
      console.log("selected series ", selected_series.title, " with imdb_id ", selected_series.id);
    }
    this.setState({
      show_final: true, 
      show_on_flow: false,
      language: this.state.language,
      found_series_commited: [selected_series],
      search_err_commited: null,
    });
    this.props.onSeriesSelect(selected_series.id, this.state.language);
  }

  handleEscPress = (e) => {
    if (e.keyCode === 27) { // esc
      if (DEBUG_LOG) {
        console.log("esc pressed")
      }
      this.setState({
        show_on_flow: false,
      });
    }
  }

  // -------------------------------------

  componentDidMount(){
    document.addEventListener("keydown", this.handleEscPress, false);
    document.addEventListener("mousemove", this.unblockMouseHover, false);
  }

  render() {
    return (
      <div>
        <div className="search-form dropdown">
          <form>
            <p className="h5 text-left grey-text">What series are you looking for?</p>
            <div className="grey-text larger-text">
              <MDBInput icon="search" group type="text" size="lg" validate 
                value={this.state.search_input}
                onChange={this.handleSearchInputChange} 
                onKeyPress={this.doFinalSearch} onKeyDown={this.handleOnTheFlowSelection}
                />
            </div>
          </form>
          <div className={"search-res-on-the-flow dropdown-content" + (this.state.show_on_flow ? " show" : "")} onMouseLeave={this.handleDropdownMouseLeave}>
            {this.state.found_series && this.state.found_series.length > 0 && this.state.found_series.map((series_item, idx) => (
              <div key={idx} data-idx={idx} className={"series-item" + (series_item.selected ? " selected" : "")} onClick={this.handleSeriesSelect} onMouseEnter={this.hadleSeriesItemHover}>
                <div className="info" data-idx={idx}>
                  <div data-idx={idx} className="title">{series_item.title}</div>
                  <div data-idx={idx} className="year">({series_item.firstAirDate})</div>
                </div>
              </div>))}
          </div>
        </div>
        {this.state.show_final && <div className="search-res">
          {this.state.found_series_commited && this.state.found_series_commited.length > 0 ? this.state.found_series_commited.map((series_item, idx) => (
            <div key={idx} data-idx={idx} 
              className={"series-item" + (series_item.not_selectable ? " not-selectable" : "")}
              onClick={this.handleSeriesCommitedSelect}>
              <div data-idx={idx} className="poster">
                <img src={series_item.poster} style={{height: 60, maxWidth: 100}}/>
              </div>
              <div className="info" data-idx={idx}>
                <div data-idx={idx} className="title">{series_item.title}</div>
                <div data-idx={idx} className="year">{series_item.firstAirDate}</div>
              </div>
            </div>)) : <div className="empty-search-res">
              {this.state.search_err_commited ? <p> {this.state.search_err_commited} </p> : <p style={{paddingLeft: '10%'}}> Nothing found </p>}
            </div>}
        </div>}
      </div>
    );
  }
}

class Episodes extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
    };
  }

  render() {
    return (
      <div className="best-episodes-list">
        {this.props.error != null ? <div className="episodes-err"> 
          {this.props.error}
        </div> : (this.props.episodes && this.props.episodes.length > 0) ? <div className="episodes-ok">
          <div className="best-episodes-title grey-text">Best of "{this.props.title}"</div>
          {this.props.episodes.map((episode, idx) => (
            <div key={idx} className="episode-item"> 
              {/* <div className="episode-idx">#{idx+1}</div> */}
              <div className="episode-info">
                <div className="episode-title" style={{display:'flex'}}>
                  <div className="episode-number">S{episode.season}E{episode.number}</div>
                  <div className="episode-title text-wrap">{episode.title}</div>
                </div>
                <div className="episode-rating"><span role="img" aria-label="star">⭐️</span> &nbsp;{episode.rating}</div>
              </div>
            </div>
          ))}
        </div> : null}
      </div>
    );
  }
}

class ApiReqs extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      reqs: [],
    }
  }

  render() {
    return (<div>
      {this.props.reqs && this.props.reqs.length > 0 && this.props.reqs.map((req, idx) => (
        <div key={idx}>
          {req.path}
          {req.resp_ip}
        </div>
      ))}
    </div>);
  }
}

class Spinner extends React.Component {
  render() {
    return (<div className="spinner">
      <div className="spinner-border" role="status">
        <span className="sr-only">Loading...</span>
      </div>
    </div>);
  }
}

class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      active_imdb_id: 0,
      active_title: "",
      active_episodes: [],
      active_error: null,
      show_api_reqs: true,
      show_spinner: false,
    }
  }

  onSeriesSelect = (series_imdb_id, language) => {
    if (DEBUG_LOG) {
      console.log("App has just known that you selected imdb_id ", series_imdb_id);
      console.log(language)
    }
    
    const doEpisodesSearch = () => {
      if (DEBUG_LOG) {
        console.log("show_spinner:", this.state.show_spinner);
      }

      const doFetch = () => {
        fetch(API_HOST+`/id/${series_imdb_id}?language=${language}`)
          .then(resp => {
            console.log("episodes resp", resp);
            if (resp.status !== 200) {
              return {
                error: `Unexpected status code ${resp.status}`,
                Title: "",
                Episodes: [],
              }
            }
            // let json = resp.json();
            return resp.json();;
          })
          .then(resp_json => {
            if (DEBUG_LOG) {
              console.log("episodes resp_json", resp_json);
            }
            if (resp_json.error == null && (resp_json.episodes == null || resp_json.episodes.length === 0)) {
              resp_json = {
                error: `We cannot find best episodes because there are no ratings on IMDb.`,
                Title: resp_json.title,
                Episodes: [],
              }
            }
            this.setState({
              active_imdb_id: series_imdb_id,
              active_episodes: resp_json.episodes,
              active_title: resp_json.title,
              active_error: resp_json.error,
            }, this.setState({
              show_spinner: false,
            }));
          });
      }
      // setTimeout(doFetch, 30000);
      doFetch();
    }
    
    this.setState({
      show_spinner: true,
    }, doEpisodesSearch);
  };

  render() {
    return (
      <div className="app-area">
        <div className="content">
          <div className="header">
            <h1 className="text-center" 
              style={{
                paddingLeft: '0.2em',
                fontWeight: 600,
                letterSpacing: '0.2em',
                /* font-family: 'Palatino Linotype,Book Antiqua,Palatino,serif'; */
                fontFamily: '"Arial Black", Gadget, sans-serif',
                // fontFamily: '"Arial", Gadget, sans-serif',
              }}>LAZY SOAP</h1>
            <p className="description text-center" 
              style={{
                color: '#dadada', 
                fontFamily: '"Arial Narrow", "Arial", sans-serif', 
                fontWeight: 200, 
                letterSpacing: '0.24em'}}>
                  watch&nbsp;only&nbsp;best episodes&nbsp;of&nbsp;series
              </p>
          </div>
          <div className="work-area">
            <div className="search-series-area">
              <SeriesSearch onSeriesSelect={this.onSeriesSelect}/>
            </div>
            {this.state.show_spinner && <Spinner/>}
            <div className="best-episodes-area">
              {(!(this.state.show_spinner && this.state.active_error != null)) &&
              <Episodes imdbId={this.state.active_imdb_id} 
                title={this.state.active_title} 
                episodes={this.state.active_episodes}
                error={this.state.active_error}/>
              }
            </div>
          </div>
        </div>
        {this.state.show_api_reqs && 
        <div className="reqs-area">
          <ApiReqs/>
        </div>}
      </div>
    );
  }
}

// ========================================

ReactDOM.render(
  <App />,
  document.getElementById('root')
);

  